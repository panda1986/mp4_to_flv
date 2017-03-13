package main

import (
    ol "github.com/ossrs/go-oryx-lib/logger"
    "fmt"
    "io"
    "reflect"
    "sort"
)

// The sample struct of mp4.
type Mp4Sample struct {
    // The type of sample, audio or video.
    sampleType int
    // The offset of sample in file.
    offset uint32
    // The index of sample with a track, start from 0.
    index uint32
    // The dts in tbn.
    dts uint64
    // For video, the pts in tbn.
    pts uint64
    // The tbn(timebase).
    tbn uint32
    // For video, the frame type, whether keyframe.
    frameType int
    // The adjust timestamp in milliseconds.
    // For example, we can adjust a timestamp for A/V to monotonically increase.
    adjust int32
    // The sample data.
    nbData uint32
    data []uint8
}

func NewMp4Sample() *Mp4Sample {
    v := &Mp4Sample{
        data: []uint8{},
    }
    return v
}

func (v *Mp4Sample) dts_ms() uint32 {
    if v.tbn > 0 {
        return uint32(int32(v.dts * 1000 / uint64(v.tbn)) + v.adjust)
    }
    return 0
}

func (v *Mp4Sample) pts_ms() uint32 {
    if v.tbn > 0 {
        return uint32(int32(v.pts * 1000 / uint64(v.tbn)) + v.adjust)
    }
    return 0
}

type Mp4SampleManager struct {
    samples []*Mp4Sample
}

func NewMp4SampleManager() *Mp4SampleManager {
    v := &Mp4SampleManager{
        samples: []*Mp4Sample{},
    }
    return v
}

func (v *Mp4SampleManager) load_trak(frameType int, track *Mp4TrackBox) (tses []*Mp4Sample, err error) {
    var mdhd *Mp4MediaHeaderBox
    var stco *Mp4ChunkOffsetBox
    var stsz *Mp4SampleSizeBox
    var stsc *Mp4Sample2ChunkBox
    var stts *Mp4DecodingTime2SampleBox
    var ctts *Mp4CompositionTime2SampleBox
    var stss *Mp4SyncSampleBox

    tses = []*Mp4Sample{}

    if mdhd, err = track.mdhd(); err != nil {
        return
    }
    if stco, err = track.stco(); err != nil {
        return
    }
    if stsz, err = track.stsz(); err != nil {
        return
    }
    if stsc, err = track.stsc(); err != nil {
        return
    }
    if stts, err = track.stts(); err != nil {
        return
    }
    if frameType == SrsFrameTypeVideo {
        ctts, _ = track.ctts()
        stss, _ = track.stss()
    }

    // Samples per chunk.
    stsc.initialize_counter()
    // DTS box
    if err = stts.initialize_counter(); err != nil {
        return
    }
    // CTS PTS box
    if ctts != nil {
        if err = ctts.initialize_counter(); err != nil {
            return
        }
    }

    var previous *Mp4Sample

    var ci uint32
    for ci = 0; ci < stco.EntryCount; ci ++ {
        // The sample offset relative in chunk.
        var sample_relative_offset uint32

        // Find how many samples from stsc.
        entry := stsc.onChunk(ci)

        var i uint32
        for i = 0; i < entry.SamplesPerChunk; i ++ {
            sample := NewMp4Sample()
            sample.sampleType = frameType
            if previous != nil {
                sample.index = previous.index + 1
            }
            sample.tbn = mdhd.TimeScale
            sample.offset = stco.Entries[ci] + sample_relative_offset

            var sampleSize uint32
            if sampleSize, err = stsz.getSampleSize(sample.index); err != nil {
                return
            }
            sample_relative_offset += sampleSize

            var sttsEntry *Mp4SttsEntry
            if sttsEntry, err = stts.on_sample(sample.index); err != nil {
                return
            }
            if previous != nil {
                sample.dts = previous.dts + uint64(sttsEntry.sampleDelta)
                sample.pts = sample.dts
            }

            var cttsEntry *Mp4CttsEntry
            if ctts != nil {
                if cttsEntry, err = ctts.on_sample(sample.index); err != nil {
                    return
                }
                sample.pts = sample.dts + uint64(cttsEntry.sampleOffset)
            }

            if frameType == SrsFrameTypeVideo {
                if stss == nil || stss.isSync(sample.index) {
                    sample.frameType = SrsVideoAvcFrameTypeKeyFrame
                } else {
                    sample.frameType = SrsVideoAvcFrameTypeInterFrame
                }
            }

            sample.nbData = sampleSize

            previous = sample
            tses = append(tses, sample)
            ol.I(nil, fmt.Sprintf("...load one sample:%+v", sample))
        }
    }
    ol.T(nil, fmt.Sprintf("total samples:%v", len(tses)))

    if previous != nil && previous.index + 1 != stsz.sampleCount {
        err = fmt.Errorf("MP4 illegal samples count, exp=%v, actual=%v", stsz.sampleCount, previous.index + 1)
        return
    }

    return
}

func (v *Mp4SampleManager) do_load(moov *Mp4MovieBox) (stss []*Mp4Sample, err error) {
    var vide *Mp4TrackBox
    if vide, err = moov.Video(); err != nil {
        return
    }
    var vstss []*Mp4Sample
    if vstss, err = v.load_trak(SrsFrameTypeVideo, vide); err != nil {
        return
    }
    ol.T(nil, fmt.Sprintf("load video trak ok, stss len=%v", len(vstss)))

    var soun *Mp4TrackBox
    if soun, err = moov.Audio(); err != nil {
        return
    }
    var astss []*Mp4Sample
    if astss, err = v.load_trak(SrsFrameTypeAudio, soun); err != nil {
        return
    }
    ol.T(nil, fmt.Sprintf("load audio trak ok, stss len=%v", len(astss)))

    stss = []*Mp4Sample{}
    stss = append(stss, vstss...)
    stss = append(stss, astss...)
    ol.T(nil, fmt.Sprintf("load trak ok, stss len=%v", len(stss)))
    return
}

type SortMp4Samples []*Mp4Sample

func (v SortMp4Samples) Len() int {
    return len(v)
}

func (v SortMp4Samples) Swap(i, j int) {
    v[i], v[j] = v[j], v[i]
}

func (v SortMp4Samples) Less(i, j int) bool {
    return int(v[i].offset) > int(v[j].offset)
}

// Load the samples from moov. There must be atleast one track.
func (v *Mp4SampleManager) load(moov *Mp4MovieBox) (err error) {
    var tses []*Mp4Sample
    if tses, err = v.do_load(moov); err != nil {
        return
    }

    // sort dict to slice
    sort.Sort(sort.Reverse(SortMp4Samples(tses)))
    ol.T(nil, fmt.Sprintf("after sort, tses len=%v, first=%+v", len(tses), tses[0]))
    // Dumps temp samples.
    // Adjust the sequence diff.
    var maxp int32
    var maxn int32

    var pvideo *Mp4Sample // the last video sample
    for k, ts := range tses {
        ol.I(nil, fmt.Sprintf("sample:%v, %+v", k, ts))
        if ts.sampleType == SrsFrameTypeVideo {
            pvideo = ts
        } else if pvideo != nil {
            // deal video and audio sample diff
            diff := int32(ts.dts_ms() - pvideo.dts_ms())
            if diff > 0 {
                maxp = max(diff, maxp)
            } else {
                maxn = min(diff, maxn)
            }
            pvideo = nil
        }
    }
    ol.T(nil, fmt.Sprintf("maxp=%v, maxn=%v", maxp, maxn))

    // Adjust when one of maxp and maxn is zero,
    // that means we can adjust by add maxn or sub maxp,
    // notice that maxn is negative and maxp is positive.
    if maxp * maxn == 0  && maxp + maxn != 0 {
        for _, ts := range tses {
            if ts.sampleType == SrsFrameTypeAudio {
                ts.adjust = 0 - maxp - maxn
            }
        }
    }

    v.samples = append(v.samples, tses...)
    return
}

/**
 * The MP4 demuxer.
 */
type Mp4Decoder struct {
    // The major brand of decoder, parse from ftyp.
    brand uint32
    // The samples build from moov.
    samples *Mp4SampleManager
    // The current written sample information.
    curIndex uint32
    // The video codec of first track, generally there is zero or one track.
    // Forbidden if no video stream.
    // TODO: FIXME: Use SrsFormat instead.
    vcodec int
    duration float64 // uint is ms
    width float64
    height float64
    vbitrate float64
    frameRate float64

    // For H.264/AVC, the avcc contains the sps/pps.
    pavcc []uint8
    // Whether avcc is written to reader.
    avccWritten bool

    // The audio codec of first track, generally there is zero or one track.
    // Forbidden if no audio stream.
    acodec int
    // The audio sample rate.
    sampleRate int
    // The audio sound bits.
    soundBits int
    // The audio sound type.
    channels int

    // For AAC, the asc in esds box.
    pasc []uint8
    // Whether asc is written to reader.
    ascWritten bool
}

func NewMp4Decoder() *Mp4Decoder {
    v := &Mp4Decoder{
        pavcc: []uint8{},
        pasc: []uint8{},
        samples: NewMp4SampleManager(),
    }
    return v
}

func (v *Mp4Decoder) Init(r io.Reader) (err error) {
    for {
        mb := NewMp4Box()
        var box Box
        if box, err = mb.discovery(r); err != nil {
            ol.E(nil, fmt.Sprintf("discovery box failed, err is %v", err))
            break
        }

        ol.T(nil, fmt.Sprintf("main discover and decode a box, type:%v", reflect.TypeOf(box)))

        if err = box.DecodeHeader(r); err != nil {
            ol.E(nil, fmt.Sprintf("mp4 decode contained box header failed, err is %v", err))
            break
        }

        if err = box.Basic().DecodeBoxes(r); err != nil {
            ol.E(nil, fmt.Sprintf("mp4 decode contained box boxes failed, err is %v", err))
            break
        }

        ol.T(nil, fmt.Sprintf("parse box, type:%v", reflect.TypeOf(box)))
        if fbox, ok := box.(*Mp4FileTypeBox); ok {
            if err = v.parseFtyp(fbox); err != nil {
                ol.E(nil, fmt.Sprintf("parse ftyp failed, err is %v", err))
                return
            }
        }else if fbox, ok := box.(*Mp4MovieBox); ok {
            if err = v.parseMoov(fbox); err != nil {
                ol.E(nil, fmt.Sprintf("parse moov failed, err is %v", err))
                return
            }
        }
    }

    if err == io.EOF {
        ol.T(nil, "init mp4 decoder success")
        return nil
    }

    return
}

func (v *Mp4Decoder) parseFtyp(box *Mp4FileTypeBox) (err error) {
    legalBrands := map[uint32]struct{}{SrsMp4BoxBrandISO2: {}, SrsMp4BoxBrandAVC1:{}, SrsMp4BoxBrandISOM:{}, SrsMp4BoxBrandMP41:{ }}
    if _, ok := legalBrands[box.majorBrand]; !ok {
        err = fmt.Errorf("Mp4 brand is illegal, brand=%v", box.majorBrand)
        ol.E(nil, err.Error())
        return
    }

    v.brand = box.majorBrand
    return
}

func (v *Mp4Decoder) parseMoov(moov *Mp4MovieBox) (err error) {
    ol.T(nil, fmt.Sprintf("...start to parse moov...."))
    var mvhd *Mp4MovieHeaderBox
    if mvhd, err = moov.Mvhd(); err != nil {
        ol.E(nil, fmt.Sprintf("mp4 missing mvhd box, err is:%v", err))
        return
    }
    v.duration = float64(mvhd.Duration())

    var vide *Mp4TrackBox
    if vide, err = moov.Video(); err != nil {
        return
    }

    var soun *Mp4TrackBox
    if soun, err = moov.Audio(); err != nil {
        return
    }

    var mp4a *Mp4AudioSampleEntry
    if mp4a, err = soun.mp4a(); err != nil {
        return
    }

    sr := mp4a.sampleRate >> 16
    if sr >= 44100 {
        v.sampleRate = SrsAudioSampleRate44100
    } else if sr >= 22050 {
        v.sampleRate = SrsAudioSampleRate22050
    } else if sr >= 11025 {
        v.sampleRate = SrsAudioSampleRate11025
    } else {
        v.sampleRate = SrsAudioSampleRate5512
    }

    if mp4a.sampleSize == 16 {
        v.soundBits = SrsAudioSampleBits16bit
    } else {
        v.soundBits = SrsAudioSampleBits8bit
    }

    if mp4a.channelCount == 2 {
        v.channels = SrsAudioChannelsStereo
    } else {
        v.channels = SrsAudioChannelsMono
    }

    var avcc *Mp4AvccBox
    if avcc, err = vide.avcc(); err != nil {
        return
    }
    var asc *Mp4DecoderSpecificInfo
    if asc, err = soun.asc(); err != nil {
        return
    }

    v.vcodec = vide.vide_codec()
    v.acodec = soun.soun_codec()

    v.pavcc = append(v.pavcc, avcc.avcConfig...)
    v.pasc = append(v.pasc, asc.asc...)

    if err = v.samples.load(moov); err != nil {
        return
    }
    // build the samples structure from moov

    ol.T(nil, fmt.Sprintf("dur=%v ms, vide=%v(%v, %v BSH),soun=%v(%v,%v BSH),%v,%v,%v", mvhd.Duration(), moov.NbVideoTracks(), v.vcodec, len(v.pavcc), moov.NbSoundTracks(), v.acodec, len(v.pasc), v.channels, v.soundBits, v.sampleRate))
    return
}

/**
 * Read a sample from mp4.
 * @param pht The sample hanler type, audio/soun or video/vide.
 * @param pft, The frame type. For video, it's SrsVideoAvcFrameType.
 * @param pct, The codec type. For video, it's SrsVideoAvcFrameTrait. For audio, it's SrsAudioAacFrameTrait.
 * @param pdts The output dts in milliseconds.
 * @param ppts The output pts in milliseconds.
 * @param pnb_sample The output size of payload.
 * @param psample The output payload, user must free it.
 * @remark The decoder will generate the first two audio/video sequence header.
 */
func (v *Mp4Decoder) readSample(mp4Url string) (s *SrsMp4Sample, err error) {
    s = NewSrsMp4Smaple()

    if !v.avccWritten && (len(v.pavcc) != 0) {
        v.avccWritten = true
        s.handlerType = SrsMp4HandlerTypeVIDE
        s.nbSample = uint32(len(v.pavcc))
        s.sample = append(s.sample, v.pavcc...)
        s.frameType = SrsVideoAvcFrameTypeKeyFrame
        s.frameTrait = SrsVideoAvcFrameTraitSequenceHeader
        ol.T(nil, fmt.Sprintf("make a video sh"))
        return
    }

    if !v.ascWritten && len(v.pasc) != 0 {
        v.ascWritten = true
        s.handlerType = SrsMp4HandlerTypeSOUN
        s.nbSample = uint32(len(v.pasc))
        s.sample = append(s.sample, v.pasc...)
        s.frameType = 0x00
        s.frameTrait = SrsAudioAacFrameTraitSequenceHeader
        ol.T(nil, fmt.Sprintf("make a audio sh"))
        return
    }

    v.curIndex ++
    if v.curIndex >= uint32(len(v.samples.samples)) {
        return nil, fmt.Errorf("sample reach end")
    }
    ms := v.samples.samples[v.curIndex]

    if ms.sampleType == SrsFrameTypeVideo {
        s.handlerType = SrsMp4HandlerTypeVIDE
        s.frameTrait = SrsVideoAvcFrameTraitNALU
    } else {
        s.handlerType = SrsMp4HandlerTypeSOUN
        s.frameTrait = SrsAudioAacFrameTraitRawData
    }

    s.dts = ms.dts_ms()
    s.pts = ms.pts_ms()
    s.frameType = uint16(ms.frameType)


    s.nbSample = ms.nbData
    var data []byte
    if data, err = readAt(mp4Url, int64(ms.offset), int(ms.nbData)); err != nil {
        return
    }
    s.sample = append(s.sample, data...)
    return
}