package main

import (
    "os"
    ol "github.com/ossrs/go-oryx-lib/logger"
    "fmt"
)

type Muxer struct {
    dec *Mp4Decoder
    mp4Url string
    flvUrl string
}

func NewMuxer(mp4, flv string) *Muxer {
    v := &Muxer{
        dec: NewMp4Decoder(),
        mp4Url: mp4,
        flvUrl: flv,
    }
    return v
}

func (v *Muxer) init() (err error) {
    var f *os.File
    if f, err = os.Open(v.mp4Url); err != nil {
        ol.E(nil, fmt.Sprintf("open mp4 file failed, err is %v", err))
        return
    }
    defer f.Close()

    if err = v.dec.Init(f); err != nil {
        ol.E(nil, fmt.Sprintf("init mp4 decoder failed, err is %v", err))
        return
    }
    ol.T(nil, fmt.Sprintf("dec:%+v", v.dec))
    return
}

func (v *Muxer) mux() (err error) {
    ol.T(nil, fmt.Sprint("start ingest mp4 to flv."))
    for {
        // Read a mp4 sample and convert to flv tag
        var s *SrsMp4Sample
        if s, err =v.readSample(); err != nil {
            return
        }

        tagType, time, data := v.sampleToFlvTag(s)
        ol.T(nil, fmt.Sprintf("tagType:%v, time:%v, len data=%v, len sample=%v", tagType, time, len(data), s.size()))
        // packet is ok.

    }
    return
}

/**
 * Read a sample form mp4.
 * @remark User can use srs_mp4_sample_to_flv_tag to convert mp4 sampel to flv tag.
 *      Use the srs_mp4_to_flv_tag_size to calc the flv tag data size to alloc.
 */
func (v *Muxer) readSample() (s *SrsMp4Sample, err error) {
    if s, err = v.dec.readSample(v.mp4Url); err != nil {
        ol.E(nil, fmt.Sprintf("read mp4 sample failed, err is %v", err))
        return
    }

    if s.handlerType == SrsMp4HandlerTypeForbidden {
        return nil, fmt.Errorf("invalid mp4 handler")
    }

    if s.handlerType == SrsMp4HandlerTypeSOUN {
        s.codec = uint16(v.dec.acodec)
        s.sampleRate = uint8(v.dec.sampleRate)
        s.channels = uint8(v.dec.channels)
        s.soundBits = uint8(v.dec.soundBits)
    } else {
        s.codec = uint16(v.dec.vcodec)
    }

    ol.I(nil, fmt.Sprintf("read a mp4 sample:%v", s))
    return
}

/**
 * Covert mp4 sample to flv tag.
 */
func (v *Muxer) sampleToFlvTag(s *SrsMp4Sample) (tagType uint8, time uint32, data []byte) {
    data = []byte{}

    time = s.dts
    if s.handlerType == SrsMp4HandlerTypeSOUN {
        tagType = SRS_RTMP_TYPE_AUDIO

        // E.4.2.1 AUDIODATA, flv_v10_1.pdf, page 3
        tmp := uint8(s.codec << 4) | uint8(s.sampleRate << 2) | uint8(s.soundBits << 1) | s.channels
        data = append(data, tmp)
        if s.codec == SrsAudioCodecIdAAC {
            if s.frameTrait == SrsAudioAacFrameTraitSequenceHeader {
                data = append(data, uint8(0))
            } else {
                data = append(data, 1)
            }
        }
        data = append(data, s.sample...)
        return
    }

    // E.4.3.1 VIDEODATA, flv_v10_1.pdf, page 5
    tmp := uint8(s.frameType << 4 | s.codec)
    data = append(data, tmp)
    if s.codec == SrsVideoCodecIdAVC {
        tagType = SRS_RTMP_TYPE_VIDEO
        if s.frameTrait == SrsVideoAvcFrameTraitSequenceHeader {
            data = append(data, uint8(0))
        } else {
            data = append(data, uint8(1))
        }
        // cts = pts - dts, where dts = flvheader->timestamp.
        cts := s.pts - s.dts // TODO: may be cts = (s.pts - s.dts) /90;
        data = append(data, to3Bytes(cts)...)
    }

    data = append(data, s.sample...)

    return
}

type SrsMp4Sample struct {
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

func NewSrsMp4Smaple() *SrsMp4Sample {
    v := &SrsMp4Sample{
        sample: []uint8{},
    }
    return v
}
/**
 * Calc the size of flv tag, for the mp4 sample to convert to.
 */
func (v *SrsMp4Sample) size() uint32 {
    if v.handlerType == SrsMp4HandlerTypeSOUN {
        if v.codec == SrsAudioCodecIdAAC {
            return v.nbSample + 2
        }
        return v.nbSample + 1
    }
    if v.codec == SrsVideoCodecIdAVC {
        return v.nbSample + 5
    }
    return v.nbSample + 1
}

func (v *SrsMp4Sample) String() string {
    return fmt.Sprintf("ht:%v, dts:%v codec:%v, frameType:%v, sampleRate:%v, soundBits:%v, channels:%v, nb=%v", v.handlerType, v.dts, v.codec, v.frameType, v.sampleRate, v.soundBits, v.channels, v.nbSample)
}