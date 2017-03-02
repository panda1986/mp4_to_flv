package main

import (
    ol "github.com/ossrs/go-oryx-lib/logger"
    "fmt"
    "io"
    "reflect"
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

type Mp4SampleManager struct {
    samples []*Mp4Sample
}

func NewMp4SampleManager() *Mp4SampleManager {
    v := &Mp4SampleManager{
        samples: []*Mp4Sample{},
    }
    return v
}

func (v *Mp4SampleManager) load_trak(frameType int, track *Mp4TrackBox) (tses map[uint32]*Mp4Sample, err error) {
    var mdhd *Mp4MediaHeaderBox
    var stco *Mp4ChunkOffsetBox
    var stsz *Mp4SampleSizeBox
    var stsc *Mp4Sample2ChunkBox
    var stts *Mp4DecodingTime2SampleBox
    var ctts *Mp4CompositionTime2SampleBox
    var stss *Mp4SyncSampleBox

    tses = make(map[uint32]*Mp4Sample)

    if mdhd, err = track.mdhd(); err != nil {
        return
    }
    tt := track.trackType()
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
            sample.sampleType = tt
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

            if tt == SrsFrameTypeVideo {
                if stss == nil || stss.isSync(sample.index) {
                    sample.frameType = SrsVideoAvcFrameTypeKeyFrame
                } else {
                    sample.frameType = SrsVideoAvcFrameTypeInterFrame
                }
            }

            sample.nbData = sampleSize

            previous = sample
            tses[sample.offset] = sample
            ol.T(nil, fmt.Sprintf("...load one sample:%+v", sample))
        }
    }
    ol.T(nil, fmt.Sprintf("total samples:%v", len(tses)))

    if previous != nil && previous.index + 1 != stsz.sampleCount {
        err = fmt.Errorf("MP4 illegal samples count, exp=%v, actual=%v", stsz.sampleCount, previous.index + 1)
        return
    }

    return
}

func (v *Mp4SampleManager) do_load(moov *Mp4MovieBox) (err error) {
    var vide *Mp4TrackBox
    if vide, err = moov.Video(); err != nil {
        return
    }
    var vstss map[uint32]*Mp4Sample
    if vstss, err = v.load_trak(SrsFrameTypeVideo, vide); err != nil {
        return
    }
    ol.T(nil, fmt.Sprintf("load video trak ok, stss len=%v", len(vstss)))

    var soun *Mp4TrackBox
    if soun, err = moov.Audio(); err != nil {
        return
    }
    var astss map[uint32]*Mp4Sample
    if astss, err = v.load_trak(SrsFrameTypeAudio, soun); err != nil {
        return
    }
    ol.T(nil, fmt.Sprintf("load audio trak ok, stss len=%v", len(astss)))
    return
}

// Load the samples from moov. There must be atleast one track.
func (v *Mp4SampleManager) load(moov *Mp4MovieBox) (err error) {
    if err = v.do_load(moov); err != nil {
        return
    }
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

    // For H.264/AVC, the avcc contains the sps/pps.
    nbAvcc int
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

    // For H.264/AVC, the avcc contains the sps/pps.
    pavcc []uint8
    // For AAC, the asc in esds box.
    pasc []uint8
    // Whether asc is written to reader.
    ascWritten bool
}

func NewMp4Decoder() *Mp4Decoder {
    v := &Mp4Decoder{
        pavcc: []uint8{},
        pasc: []uint8{},
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

func (v *Mp4Decoder) readSample(r io.Reader) (err error) {
    // Read sample from io, for we never preload the samples(too large)
    return
}