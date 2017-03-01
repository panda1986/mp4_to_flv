package main

import (
    ol "github.com/ossrs/go-oryx-lib/logger"
    "fmt"
    "io"
    "reflect"
)

// The sample struct of mp4.
type Mp4Sample struct {
    // The handler type, it's SrsMp4HandlerType.
    handlerType uint32

    // The dts in milliseconds.
    dts uint32
    // The codec id.
    //      video: SrsVideoCodecId.
    //      audio: SrsAudioCodecId.
    codec uint16
    // The frame trait, some characteristic:
    //      video: SrsVideoAvcFrameTrait.
    //      audio: SrsAudioAacFrameTrait.
    frameTrait uint16

    // The video pts in milliseconds. Ignore for audio.
    pts uint32
    // The video frame type, it's SrsVideoAvcFrameType.
    frameType uint16

    // The audio sample rate, it's SrsAudioSampleRate.
    sampleRate uint8
    // The audio sound bits, it's SrsAudioSampleBits.
    soundBits uint8
    // The audio sound type, it's SrsAudioChannels.
    channels uint8

    // The size of sample payload in bytes.
    nbSample uint32
    // The output sample data, user must free it by srs_mp4_free_sample.
    sample []uint8
}

func NewMp4Sample() *Mp4Sample {
    v := &Mp4Sample{
        sample: []uint8{},
    }
    return v
}

type Mp4SampleManager struct {

}

func (v *Mp4SampleManager) load(box *Mp4MovieBox) (err error) {
    return
}

func (v *Mp4SampleManager) at(index int) *Mp4Sample {
    return nil
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

    ol.T(nil, fmt.Sprintf("dur=%v ms, vide=%v(%v, %v BSH),soun=%v(%v,%v BSH),%v,%v,%v", mvhd.Duration(), moov.NbVideoTracks(), v.vcodec, len(v.pavcc), moov.NbSoundTracks(), v.acodec, len(v.pasc), v.channels, v.soundBits, v.sampleRate))
    return
}

func (v *Mp4Decoder) readSample(r io.Reader) (err error) {
    // Read sample from io, for we never preload the samples(too large)
    return
}