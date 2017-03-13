package main

import (
    "fmt"
    "io"
    ol "github.com/ossrs/go-oryx-lib/logger"
    "encoding/binary"
    "reflect"
)

type Box interface {
    Basic() *Mp4Box
    NbHeader() int
    DecodeHeader(r io.Reader) (err error)
}

type Mp4Box struct {
    // The size is the entire size of the box, including the size and type header, fields,
    // and all contained boxes. This facilitates general parsing of the file.
    //
    // if size is 1 then the actual size is in the field largesize;
    // if size is 0, then this box is the last one in the file, and its contents
    // extend to the end of the file (normally only used for a Media Data Box)
    SmallSize uint32
    LargeSize uint64

    // For box 'uuid'.
    UserType  [16]uint8

    // The position at buffer to start demux the box.
    StartPos  int

    // identifies the box type; standard boxes use a compact type, which is normally four printable
    // characters, to permit ease of identification, and is shown so in the boxes below. User extensions use
    // an extended type; in this case, the type field is set to ‘uuid’.
    BoxType   uint32

    Boxes     []Box
    UsedSize  uint64
}

func NewMp4Box() *Mp4Box {
    v := &Mp4Box{}
    v.BoxType = SrsMp4BoxTypeForbidden
    v.Boxes = []Box{}
    v.UserType = [16]uint8{}
    return v
}

// Get the size of box, whatever small or large size.
func (v *Mp4Box) sz() uint64 {
    if v.SmallSize == SRS_MP4_USE_LARGE_SIZE {
        return v.LargeSize
    }
    return uint64(v.SmallSize)
}

func (v *Mp4Box) left() uint64 {
    ol.I(nil, "left:", v.sz(), v.UsedSize)
    return v.sz() - v.UsedSize
}

// Get the contained box of specific type.
// @return The first matched box.
func (v *Mp4Box) get(bt uint32) (Box, error) {
    for _, box := range v.Boxes {
        if box.Basic().BoxType == bt {
            return box, nil
        }
    }
    return nil, fmt.Errorf("can't find bt:%x in boxes", bt)
}


// Remove the contained box of specified type.
// @return The removed count.
func (v *Mp4Box) remove(bt uint32) (nbRemoved int) {
    for k, box := range v.Boxes {
        if box.Basic().BoxType == bt {
            v.Boxes = append(v.Boxes[:k], v.Boxes[k+1:]...)
            nbRemoved ++
        }
    }
    return
}

func (v *Mp4Box) NbHeader() int {
    size := 8
    if v.SmallSize == SRS_MP4_USE_LARGE_SIZE {
        size += 8
    }
    if v.BoxType == SrsMp4BoxTypeUUID {
        size += 16
    }
    return size
}

func (v *Mp4Box) DecodeHeader(r io.Reader) (err error) {
    return
}

func (v *Mp4Box) discovery(r io.Reader) (box Box, err error) {
    v.UsedSize = 0

    // Discovery the size and type.
    var largeSize uint64
    var smallSize uint32

    if err = v.Read(r, &smallSize); err != nil {
        ol.E(nil, fmt.Sprintf("read small size failed, err is %v", err))
        return
    }

    var bt uint32
    if err = v.Read(r, &bt); err != nil {
        ol.E(nil, fmt.Sprintf("read type failed, err is %v", err))
        return
    }

    if smallSize == SRS_MP4_USE_LARGE_SIZE {
        if err = v.Read(r, &largeSize); err != nil {
            ol.E(nil, fmt.Sprintf("read large size failed, err is %v", err))
            return
        }
    }

    // Only support 31bits size.
    if (largeSize > 0x7fffffff) {
        err = fmt.Errorf("box overflow")
        ol.E(nil, err.Error())
        return
    }

    switch bt {
    case SrsMp4BoxTypeFTYP:
        box = NewMp4FileTypeBox()
    case SrsMp4BoxTypeMOOV:
        box = NewMp4MovieBox()
    case SrsMp4BoxTypeMVHD:
        box = NewMp4MovieHeaderBox()
    case SrsMp4BoxTypeTRAK:
        box = &Mp4TrackBox{}
    case SrsMp4BoxTypeTKHD:
        box = NewMp4TrackHeaderBox()
    case SrsMp4BoxTypeMDIA:
        box = &Mp4MediaBox{}
    case SrsMp4BoxTypeMDHD:
        box = &Mp4MediaHeaderBox{}
    case SrsMp4BoxTypeHDLR:
        box = NewMp4HandlerReferenceBox()
    case SrsMp4BoxTypeMINF:
        box = &Mp4MediaInformationBox{}
    case SrsMp4BoxTypeVMHD:
        box = NewMp4VideoMediaHeaderBox()
    case SrsMp4BoxTypeDINF:
        box = &Mp4DataInformationBox{}
    case SrsMp4BoxTypeSTBL:
        box = &Mp4SampleTableBox{}

    case SrsMp4BoxTypeAVC1:
        box = NewMp4VisualSampleEntry()
    case SrsMp4BoxTypeAVCC:
        box = &Mp4AvccBox{}
    case SrsMp4BoxTypeMP4A:
        box = &Mp4AudioSampleEntry{}
    case SrsMp4BoxTypeESDS:
        box = NewMp4EsdsBox()

    case SrsMp4BoxTypeSTSD:
        box = NewMp4SampleDescritionBox()
    case SrsMp4BoxTypeSTTS:
        box = NewMp4DecodingTime2SampleBox()
    case SrsMp4BoxTypeCTTS:
        box = NewMp4CompositionTime2SampleBox()
    case SrsMp4BoxTypeSTSS:
        box = NewMp4SyncSampleBox()
    case SrsMp4BoxTypeSTSC:
        box = NewMp4Sample2ChunkBox()
    case SrsMp4BoxTypeSTSZ:
        box = NewMp4SampleSizeBox()
    case SrsMp4BoxTypeSTCO:
        box = NewMp4ChunkOffsetBox()
    case SrsMp4BoxTypeUDTA:
        box = NewMp4UserDataBox()
    case SrsMp4BoxTypeMDAT:
        box = NewMp4MediaDataBox()
    default:
        box = NewMp4FreeSpaceBox()
    }

    box.Basic().BoxType = bt
    box.Basic().SmallSize = smallSize
    box.Basic().LargeSize = largeSize
    box.Basic().UsedSize = v.UsedSize

    ol.I(nil, fmt.Sprintf("discovery a new box:%v small size=%v, large size=%v, bt=%x", reflect.TypeOf(box), smallSize, largeSize, bt))
    return
}

func (v *Mp4Box) DecodeBoxes(r io.Reader) (err error) {
    // read left space
    left := v.left()
    ol.I(nil, fmt.Sprintf("after decode header, left space:%v", left))
    for {
        if left <= 0 {
            break
        }

        var box Box
        if box, err = v.discovery(r); err != nil {
            ol.E(nil, fmt.Sprintf("mp4 discovery contained box failed, err is %v", err))
            return
        }

        if err = box.DecodeHeader(r); err != nil {
            ol.E(nil, fmt.Sprintf("mp4 decode contained box header failed, err is %v", err))
            return
        }
        if err = box.Basic().DecodeBoxes(r); err != nil {
            ol.E(nil, fmt.Sprintf("mp4 decode contained box boxes failed, err is %v", err))
            return
        }

        ol.I(nil, fmt.Sprintf("box:%v decode boxes success, sub boxes=%v, box.sz=%v, left=%v %v.", reflect.TypeOf(box), len(box.Basic().Boxes), box.Basic().sz(), left, left - box.Basic().sz()))

        v.Boxes = append(v.Boxes, box)

        left -= box.Basic().sz()
    }
    return
}

func (v *Mp4Box) Skip(r io.Reader, num uint64) {
    if num <= 0 {
        return
    }

    data := make([]uint8, num)
    v.Read(r, data)
    //v.UsedSize += num
    ol.I(nil, fmt.Sprintf("skip %v bytes", num))
}

func (v *Mp4Box) Read(r io.Reader, data interface{}) (err error) {
    if err = binary.Read(r, binary.BigEndian, data); err != nil {
        return 
    }
    v.UsedSize += uint64DataSize(data)
    return 
}

type Mp4FreeSpaceBox struct {
    Mp4Box
    needSkip int
}

func NewMp4FreeSpaceBox() *Mp4FreeSpaceBox {
    v := &Mp4FreeSpaceBox{
    }
    return v
}

func (v *Mp4FreeSpaceBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4FreeSpaceBox) NbHeader() int {
    return v.Mp4Box.NbHeader() + v.needSkip
}

func (v *Mp4FreeSpaceBox) DecodeHeader(r io.Reader) (err error) {
    v.needSkip = int(v.left())
    v.Skip(r, v.left())
    return
}

// ftyp box
type Mp4FileTypeBox struct {
    Mp4Box
    majorBrand uint32
    minorVersion uint32
    compatibleBrands []uint32
}

func NewMp4FileTypeBox() *Mp4FileTypeBox {
    v := &Mp4FileTypeBox{
        majorBrand: SrsMp4BoxBrandForbidden,
        minorVersion: 0,
        compatibleBrands: []uint32{},
    }
    return v
}

func (v *Mp4FileTypeBox) setCompatibleBrands(b0, b1, b2, b3 uint32) {
    v.compatibleBrands = append(v.compatibleBrands, []uint32{b0, b1, b2, b3}...)
}

func (v *Mp4FileTypeBox) DecodeHeader(r io.Reader) (err error) {
    /*if err = v.Mp4Box.DecodeHeader(r); err != nil {
        return err
    }*/

    ol.I(nil, fmt.Sprintf("decode ftyp box, usedSize=%v", v.UsedSize))
    if err = v.Read(r, &v.majorBrand); err != nil {
        ol.E(nil, fmt.Sprintf("read major brand failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.minorVersion); err != nil {
        ol.E(nil, fmt.Sprintf("read minor version failed, err is %v", err))
        return
    }

    // Compatible brands to the end of the box.
    left := v.Mp4Box.left()
    if (left > 0) {
        for i := 0; i < int(left) / 4; i ++ {
            var brand uint32
            if err = v.Read(r, &brand); err != nil {
                ol.E(nil, fmt.Sprintf("read brand failed, err is %v", err))
                return
            }
            v.compatibleBrands = append(v.compatibleBrands, brand)
        }
    }
    return
}

func (v *Mp4FileTypeBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4FileTypeBox) NbHeader() int {
    return v.Mp4Box.NbHeader() + 8 + len(v.compatibleBrands) * 4
}

/**
 * 8.2.1 Movie Box (moov)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 30
 * The metadata for a presentation is stored in the single Movie Box which occurs at the top-level of a file.
 * Normally this box is close to the beginning or end of the file, though this is not required.
 */
// moov box
type Mp4MovieBox struct {
    Mp4Box
}

func NewMp4MovieBox() *Mp4MovieBox {
    v := &Mp4MovieBox{}
    return v
}

// Get the header of moov.
func (v *Mp4MovieBox) Mvhd() (*Mp4MovieHeaderBox, error) {
    if box, err := v.get(SrsMp4BoxTypeMVHD); err != nil {
        return nil, err
    } else {
        return box.(*Mp4MovieHeaderBox), nil
    }
}

func (v *Mp4MovieBox) Video() (*Mp4TrackBox, error) {
    for _, box := range v.Boxes {
        if tbox, ok := box.(*Mp4TrackBox); ok {
            if tbox.trackType() == SrsMp4TrackTypeVideo {
                return tbox, nil
            }
        }
    }
    return nil, fmt.Errorf("can't find video trak box in moov")
}

func (v *Mp4MovieBox) Audio() (*Mp4TrackBox, error) {
    for _, box := range v.Boxes {
        if tbox, ok := box.(*Mp4TrackBox); ok {
            if tbox.trackType() == SrsMp4TrackTypeAudio {
                return tbox, nil
            }
        }
    }
    return nil, fmt.Errorf("can't find audio trak box in moov")
}

// Get the number of video tracks
func (v *Mp4MovieBox) NbVideoTracks() (nb_tracks int) {
    for _, box := range v.Boxes {
        if tbox, ok := box.(*Mp4TrackBox); ok {
            if tbox.trackType() == SrsMp4TrackTypeVideo {
                nb_tracks ++
            }
        }
    }
    return
}

// Get the number of audio tracks
func (v *Mp4MovieBox) NbSoundTracks() (nb_tracks int) {
    for _, box := range v.Boxes {
        if tbox, ok := box.(*Mp4TrackBox); ok {
            if tbox.trackType() == SrsMp4TrackTypeAudio {
                nb_tracks ++
            }
        }
    }
    return
}

func (v *Mp4MovieBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4MovieBox) NbHeader() int {
    return v.Mp4Box.NbHeader()
}

/**
 * 4.2 Object Structure
 * ISO_IEC_14496-12-base-format-2012.pdf, page 17
 */
type Mp4FullBox struct {
    Mp4Box
    Version uint8
    Flags uint32
}

func (v *Mp4FullBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4FullBox) NbHeader() int {
    return v.Mp4Box.NbHeader() + 1 + 3
}

func (v *Mp4FullBox) DecodeHeader(r io.Reader) (err error) {
    /*if err = v.Mp4Box.DecodeHeader(r); err != nil {
        return
    }*/

    if err = v.Read(r, &v.Flags); err != nil {
        ol.E(nil, fmt.Sprintf("read moov flags failed, err is %v", err))
        return
    }

    v.Version = uint8((v.Flags >> 24) & 0xff)
    v.Flags = v.Flags & 0x00ffffff

    return
}

/**
 * 8.2.2 Movie Header Box (mvhd)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 31
 */
type Mp4MovieHeaderBox struct {
    Mp4FullBox
    // an integer that declares the creation time of the presentation (in seconds since
    // midnight, Jan. 1, 1904, in UTC time)
    CreateTime uint64
    // an integer that declares the most recent time the presentation was modified (in
    // seconds since midnight, Jan. 1, 1904, in UTC time)
    ModTime uint64
    // an integer that specifies the time-scale for the entire presentation; this is the number of
    // time units that pass in one second. For example, a time coordinate system that measures time in
    // sixtieths of a second has a time scale of 60.
    TimeScale uint32
    // an integer that declares length of the presentation (in the indicated timescale). This property
    // is derived from the presentation’s tracks: the value of this field corresponds to the duration of the
    // longest track in the presentation. If the duration cannot be determined then duration is set to all 1s.
    DurationInTbn uint64
    // a fixed point 16.16 number that indicates the preferred rate to play the presentation; 1.0
    // (0x00010000) is normal forward playback
    Rate uint32
    // a fixed point 8.8 number that indicates the preferred playback volume. 1.0 (0x0100) is full volume.
    Volume uint16
    Reserved0 uint16
    Reserved1 uint64
    // a transformation matrix for the video; (u,v,w) are restricted here to (0,0,1), hex values (0,0,0x40000000).
    Matrix [9]int32
    PreDefined [6]uint32
    // a non-zero integer that indicates a value to use for the track ID of the next track to be
    // added to this presentation. Zero is not a valid track ID value. The value of next_track_ID shall be
    // larger than the largest track-ID in use. If this value is equal to all 1s (32-bit maxint), and a new media
    // track is to be added, then a search must be made in the file for an unused track identifier.
    NextTrackId uint32
}

func NewMp4MovieHeaderBox() *Mp4MovieHeaderBox {
    v := &Mp4MovieHeaderBox{
        Matrix: [9]int32{},
        PreDefined: [6]uint32{},
    }
    return v
}

// Get the duration in ms
func (v *Mp4MovieHeaderBox) Duration() uint64 {
    if v.TimeScale > 0 {
        return v.DurationInTbn * 1000 / uint64(v.TimeScale)
    }
    return 0
}

func (v *Mp4MovieHeaderBox) Basic() *Mp4Box {
    return &v.Mp4FullBox.Mp4Box
}

func (v *Mp4MovieHeaderBox) NbHeader() int {
    return 0
}

func (v *Mp4MovieHeaderBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if v.Version == 1 {
        if err = v.Read(r, &v.CreateTime); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd create time failed, err is %v", err))
            return
        }

        if err = v.Read(r, &v.ModTime); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd mod time failed, err is %v", err))
            return
        }

        if err = v.Read(r, &v.TimeScale); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd time scale failed, err is %v", err))
            return
        }

        if err = v.Read(r, &v.DurationInTbn); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd duration failed, err is %v", err))
            return
        }
    } else {
        var tmp uint32
        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd create time failed, err is %v", err))
            return
        }
        v.CreateTime = uint64(tmp)

        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd mod time failed, err is %v", err))
            return
        }
        v.ModTime = uint64(tmp)

        if err = v.Read(r, &v.TimeScale); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd time scale failed, err is %v", err))
            return
        }

        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd duration failed, err is %v", err))
            return
        }
        v.DurationInTbn = uint64(tmp)
    }

    if err = v.Read(r, &v.Rate); err != nil {
        ol.E(nil, fmt.Sprintf("read mvhd rate failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.Volume); err != nil {
        ol.E(nil, fmt.Sprintf("read mvhd volume failed, err is %v", err))
        return
    }

    v.Skip(r, v.left())

    return
}

/**
 * 8.3.1 Track Box (trak)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 32
 * This is a container box for a single track of a presentation. A presentation consists of one or more tracks.
 * Each track is independent of the other tracks in the presentation and carries its own temporal and spatial
 * information. Each track will contain its associated Media Box.
 */
type Mp4TrackBox struct {
    Mp4Box
}

func (v *Mp4TrackBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4TrackBox) NdHeader() int {
    return v.Mp4Box.NbHeader()
}

func (v *Mp4TrackBox) vide_codec() (codec int) {
    codec = SrsVideoCodecIdForbidden
    if box, err := v.stsd(); err != nil {
        return
    } else if len(box.Entries) == 0 {
        return
    } else {
        entry := box.Entries[0]
        if _, ok := entry.(*Mp4VisualSampleEntry); ok {
            codec = SrsVideoCodecIdAVC
        }
    }
    return
}

func (v *Mp4TrackBox) soun_codec() (codec int) {
    codec = SrsAudioCodecIdForbidden
    if box, err := v.stsd(); err != nil {
        return
    } else if len(box.Entries) == 0 {
        return
    } else {
        entry := box.Entries[0]
        if _, ok := entry.(*Mp4AudioSampleEntry); ok {
            codec = SrsAudioCodecIdAAC
        }
    }
    return
}

func (v *Mp4TrackBox) trackType() int {
    if box, err := v.get(SrsMp4BoxTypeMDIA); err != nil {
        return SrsMp4TrackTypeForbidden
    } else {
        mdia := box.(*Mp4MediaBox)
        return mdia.trackType()
    }
}

func (v *Mp4TrackBox) stsc() (*Mp4Sample2ChunkBox, error) {
    if box, err := v.stbl(); err != nil {
        return nil, err
    } else {
        return box.stsc()
    }
}

func (v *Mp4TrackBox) stts() (*Mp4DecodingTime2SampleBox, error) {
    if box, err := v.stbl(); err != nil {
        return nil, err
    } else {
        return box.stts()
    }
}

func (v *Mp4TrackBox) ctts() (*Mp4CompositionTime2SampleBox, error) {
    if box, err := v.stbl(); err != nil {
        return nil, err
    } else {
        return box.ctts()
    }
}

func (v *Mp4TrackBox) stsz() (*Mp4SampleSizeBox, error) {
    if box, err := v.stbl(); err != nil {
        return nil, err
    } else {
        return box.stsz()
    }
}

func (v *Mp4TrackBox) stss() (*Mp4SyncSampleBox, error) {
    if box, err := v.stbl(); err != nil {
        return nil, err
    } else {
        return box.stss()
    }
}

func (v *Mp4TrackBox) stco() (*Mp4ChunkOffsetBox, error) {
    if box, err := v.stbl(); err != nil {
        return nil, err
    } else {
        return box.stco()
    }
}

func (v *Mp4TrackBox) mdhd() (*Mp4MediaHeaderBox, error) {
    if box, err := v.mdia(); err != nil {
        return nil, err
    } else {
        return box.mdhd()
    }
}

func (v *Mp4TrackBox) mdia() (*Mp4MediaBox, error) {
    if box, err := v.get(SrsMp4BoxTypeMDIA); err != nil {
        return nil, err
    } else {
        return box.(*Mp4MediaBox), nil
    }
}

func (v *Mp4TrackBox) minf() (*Mp4MediaInformationBox, error) {
    if box, err := v.mdia(); err != nil {
        return nil, err
    } else {
        return box.minf()
    }
}

func (v *Mp4TrackBox) stbl() (*Mp4SampleTableBox, error) {
    if box, err := v.minf(); err != nil {
        return nil, err
    } else {
        return box.stbl()
    }
}

func (v *Mp4TrackBox) stsd() (*Mp4SampleDescritionBox, error) {
    if box, err := v.stbl(); err != nil {
        return nil, err
    } else {
        return box.stsd()
    }
}

func (v *Mp4TrackBox) mp4a() (*Mp4AudioSampleEntry, error) {
    if box, err := v.stsd(); err != nil {
        return nil, err
    } else {
        return box.mp4a()
    }
}

func (v *Mp4TrackBox) avc1() (*Mp4VisualSampleEntry, error) {
    if box, err := v.stsd(); err != nil {
        return nil, err
    } else {
        return box.avc1()
    }
}

func (v *Mp4TrackBox) avcc() (*Mp4AvccBox, error) {
    if box, err := v.avc1(); err != nil {
        return nil, err
    } else {
        return box.avcc()
    }
}

func (v *Mp4TrackBox) asc() (*Mp4DecoderSpecificInfo, error) {
    if box, err := v.mp4a(); err != nil {
        return nil, err
    } else {
        return box.asc()
    }
}

/**
 * 8.3.2 Track Header Box (tkhd)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 32
 */
type Mp4TrackHeaderBox struct {
    Mp4FullBox
    // an integer that declares the creation time of the presentation (in seconds since
    // midnight, Jan. 1, 1904, in UTC time)
    CreateTime uint64
    ModTime uint64
    TrackId uint32
    Reserved0 uint32
    Duration uint64
    Reserved1 uint64
    Layer int16
    AlternateGroup int16
    Volume int16
    Reserved2 uint16
    Matrix [9]int32
    Width int32
    Height int32
}

func NewMp4TrackHeaderBox() *Mp4TrackHeaderBox {
    v := &Mp4TrackHeaderBox{
        Matrix: [9]int32{0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000},
    }
    v.Flags = 0x03
    return v
}

func (v *Mp4TrackHeaderBox) Basic() *Mp4Box {
    return &v.Mp4FullBox.Mp4Box
}

func (v *Mp4TrackHeaderBox) NbHeader() int {
    return v.Mp4FullBox.NbHeader()
}

func (v *Mp4TrackHeaderBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if v.Version == 1 {
        if err = v.Read(r, &v.CreateTime); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read create time failed, err is %v", err))
            return 
        }
        
        if err = v.Read(r, &v.ModTime); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read mod time failed, err is %v", err))
            return
        }
        
        if err = v.Read(r, &v.TrackId); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read track id failed, err is %v", err))
            return
        }
        
        v.Skip(r, uint64(4))
        
        if err = v.Read(r, &v.Duration); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read duration failed, err is %v", err))
            return
        }
    } else {
        var tmp uint32
        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read create time failed, err is %v", err))
            return
        }
        v.CreateTime = uint64(tmp)

        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd mod time failed, err is %v", err))
            return
        }
        v.ModTime = uint64(tmp)
        
        if err = v.Read(r, &v.TrackId); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read track id failed, err is %v", err))
            return
        }

        v.Skip(r, uint64(4))

        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read duration failed, err is %v", err))
            return
        }
        v.Duration = uint64(tmp)
    }
    
    v.Skip(r, uint64(8))
    if err = v.Read(r, &v.Layer); err != nil {
        ol.E(nil, fmt.Sprintf("read tkhd layer failed, err is %v", err))
        return 
    }
    
    if err = v.Read(r, &v.AlternateGroup); err != nil {
        ol.E(nil, fmt.Sprintf("read tkhd alternate froup failed, err is %v", err))
        return 
    }
    
    if err = v.Read(r, &v.Volume); err != nil {
        ol.E(nil, fmt.Sprintf("read tkhd volume failed, err is %v", err))
        return
    }

    v.Skip(r, uint64(2))

    for i := 0; i < len(v.Matrix); i ++ {
        if err = v.Read(r, &v.Matrix[i]); err != nil {
            ol.E(nil, fmt.Sprintf("read tkhd matrix %d failed, err is %v", i, err))
            return
        }
    }

    //TODO: width and height is 16.16 format, need to be convert
    if err = v.Read(r, &v.Width); err != nil {
        ol.E(nil, fmt.Sprintf("read tkhd width failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.Height); err != nil {
        ol.E(nil, fmt.Sprintf("read tkhd height failed, err is %v", err))
        return
    }

    ol.I(nil, fmt.Sprintf("decode tkhd:%+v", v))
    return
}

/**
 * 8.4.1 Media Box (mdia)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 36
 * The media declaration container contains all the objects that declare information about the media data within a
 * track.
 */
type Mp4MediaBox struct {
    Mp4Box
}

func (v *Mp4MediaBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4MediaBox) mdhd() (*Mp4MediaHeaderBox, error) {
    if box, err := v.get(SrsMp4BoxTypeMDHD); err != nil {
        return nil, err
    } else {
        return box.(*Mp4MediaHeaderBox), nil
    }
}

func (v *Mp4MediaBox) minf() (*Mp4MediaInformationBox, error) {
    if box, err := v.get(SrsMp4BoxTypeMINF); err != nil {
        return nil, err
    } else {
        return box.(*Mp4MediaInformationBox), nil
    }
}

func (v *Mp4MediaBox) trackType() int {
    if box, err := v.get(SrsMp4BoxTypeHDLR); err != nil {
        return SrsMp4TrackTypeForbidden
    } else {
        hdlr := box.(*Mp4HandlerReferenceBox)
        if hdlr.HandlerType == SrsMp4HandlerTypeSOUN {
            return SrsMp4TrackTypeAudio
        }
        if hdlr.HandlerType == SrsMp4HandlerTypeVIDE {
            return SrsMp4TrackTypeVideo
        }
    }
    return SrsMp4TrackTypeForbidden
}

/**
 * 8.4.2 Media Header Box (mdhd)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 36
 * The media declaration container contains all the objects that declare information about the media data within a
 * track.
 */
type Mp4MediaHeaderBox struct {
    Mp4FullBox
    // an integer that declares the creation time of the presentation (in seconds since
    // midnight, Jan. 1, 1904, in UTC time)
    CreateTime uint64
    // an integer that declares the most recent time the presentation was modified (in
    // seconds since midnight, Jan. 1, 1904, in UTC time)
    ModTime    uint64
    // an integer that specifies the time-scale for the entire presentation; this is the number of
    // time units that pass in one second. For example, a time coordinate system that measures time in
    // sixtieths of a second has a time scale of 60.
    TimeScale  uint32
    // an integer that declares length of the presentation (in the indicated timescale). This property
    // is derived from the presentation’s tracks: the value of this field corresponds to the duration of the
    // longest track in the presentation. If the duration cannot be determined then duration is set to all 1s.
    Duration   uint64
    // the language code for this media. See ISO 639-2/T for the set of three character
    // codes. Each character is packed as the difference between its ASCII value and 0x60. Since the code
    // is confined to being three lower-case letters, these values are strictly positive.
    Language   uint16
    PreDefined uint16
}

func (v *Mp4MediaHeaderBox) Basic() *Mp4Box {
    return &v.Mp4FullBox.Mp4Box
}

func (v *Mp4MediaHeaderBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if v.Version == 1 {
        if err = v.Read(r, &v.CreateTime); err != nil {
            ol.E(nil, fmt.Sprintf("mdhd read create time failed, err is %v", err))
            return
        }

        if err = v.Read(r, &v.ModTime); err != nil {
            ol.E(nil, fmt.Sprintf("mdhd read mod time failed, err is %v", err))
            return
        }

        if err = v.Read(r, &v.TimeScale); err != nil {
            ol.E(nil, fmt.Sprintf("mdhd read timescale failed, err is %v", err))
            return
        }

        if err = v.Read(r, &v.Duration); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read duration failed, err is %v", err))
            return
        }
    } else {
        var tmp uint32
        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("mdhd read create time failed, err is %v", err))
            return
        }
        v.CreateTime = uint64(tmp)

        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("mdhd mod time failed, err is %v", err))
            return
        }
        v.ModTime = uint64(tmp)

        if err = v.Read(r, &v.TimeScale); err != nil {
            ol.E(nil, fmt.Sprintf("mdhd read time scale failed, err is %v", err))
            return
        }

        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("mdhd read duration failed, err is %v", err))
            return
        }
        v.Duration = uint64(tmp)
    }

    if err = v.Read(r, &v.Language); err != nil {
        ol.E(nil, fmt.Sprintf("mdhd read language failed, err is %v", err))
        return
    }
    v.Skip(r, uint64(2))

    ol.I(nil, fmt.Sprintf("decode mdhd bos success, box:%+v", v))
    return
}

/**
 * 8.4.3 Handler Reference Box (hdlr)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 37
 * This box within a Media Box declares the process by which the media-data in the track is presented, and thus,
 * the nature of the media in a track. For example, a video track would be handled by a video handler.
 */
type Mp4HandlerReferenceBox struct {
    Mp4FullBox
    PreDefined uint32
    // an integer containing one of the following values, or a value from a derived specification:
    //      ‘vide’, Video track
    //      ‘soun’, Audio track
    HandlerType uint32
    Reserved [3]uint32
    // a null-terminated string in UTF-8 characters which gives a human-readable name for the track
    // type (for debugging and inspection purposes).
    Name string
}

func NewMp4HandlerReferenceBox() *Mp4HandlerReferenceBox {
    v := &Mp4HandlerReferenceBox{
        Reserved: [3]uint32{},
    }
    return v
}

func (v *Mp4HandlerReferenceBox) Basic() *Mp4Box {
    return &v.Mp4FullBox.Mp4Box
}

func (v *Mp4HandlerReferenceBox) NbHeader() int {
    return v.Mp4FullBox.NbHeader()
}

func (v *Mp4HandlerReferenceBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    v.Skip(r, uint64(4))

    if err = v.Read(r, &v.HandlerType); err != nil {
        ol.E(nil, fmt.Sprintf("read hdlr handler type failed, err is %v", err))
        return
    }

    v.Skip(r, uint64(12))

    data := make([]uint8, v.left())
    if err = v.Read(r, data); err != nil {
        ol.E(nil, fmt.Sprintf("read hdlr name failed, err is %v", err))
        return
    }
    v.Name = string(data)

    ol.I(nil, fmt.Sprintf("decode hdlr box success, box:%+v", v))
    return
}

/**
 * 8.4.4 Media Information Box (minf)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 38
 * This box contains all the objects that declare characteristic information of the media in the track.
 */
type Mp4MediaInformationBox struct {
    Mp4Box
}

func (v *Mp4MediaInformationBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4MediaInformationBox) stbl() (*Mp4SampleTableBox, error) {
    if box, err := v.get(SrsMp4BoxTypeSTBL); err != nil {
        return nil, err
    } else {
        return box.(*Mp4SampleTableBox), nil
    }
}

/**
 * 8.4.5.2 Video Media Header Box (vmhd)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 38
 * The video media header contains general presentation information, independent of the coding, for video
 * media. Note that the flags field has the value 1.
 */
type Mp4VideoMediaHeaderBox struct {
    Mp4FullBox
    GraphicsMode uint16
    Opcolor [3]uint16
}

func NewMp4VideoMediaHeaderBox() *Mp4VideoMediaHeaderBox {
    v := &Mp4VideoMediaHeaderBox{
        Opcolor: [3]uint16{},
    }
    v.Mp4FullBox.Version = 0
    v.Mp4FullBox.Flags = 1
    return v
}

func (v *Mp4VideoMediaHeaderBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4VideoMediaHeaderBox) NbHeader() int {
    return v.Mp4FullBox.NbHeader()
}

func (v *Mp4VideoMediaHeaderBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if err = v.Read(r, &v.GraphicsMode); err != nil {
        ol.E(nil, fmt.Sprintf("read vmhd graphics mode failed, err is %v", err))
        return
    }

    err = v.Read(r, &v.Opcolor[0])
    err = v.Read(r, &v.Opcolor[1])
    err = v.Read(r, &v.Opcolor[2])

    ol.I(nil, fmt.Sprintf("decode vmhd box success, box:%+v", v))
    return
}

/**
 * 8.7.1 Data Information Box (dinf)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 56
 * The data information box contains objects that declare the location of the media information in a track.
 */
type Mp4DataInformationBox struct {
    Mp4Box
}

func (v *Mp4DataInformationBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

/**
 * 8.5.1 Sample Table Box (stbl)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 40
 * The sample table contains all the time and data indexing of the media samples in a track. Using the tables
 * here, it is possible to locate samples in time, determine their type (e.g. I-frame or not), and determine their
 * size, container, and offset into that container.
 */
type Mp4SampleTableBox struct {
    Mp4Box
}

func (v *Mp4SampleTableBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4SampleTableBox) stsc() (*Mp4Sample2ChunkBox, error) {
    if box, err := v.get(SrsMp4BoxTypeSTSC); err != nil {
        return nil, err
    } else {
        return box.(*Mp4Sample2ChunkBox), nil
    }
}

func (v *Mp4SampleTableBox) stts() (*Mp4DecodingTime2SampleBox, error) {
    if box, err := v.get(SrsMp4BoxTypeSTTS); err != nil {
        return nil, err
    } else {
        return box.(*Mp4DecodingTime2SampleBox), nil
    }
}

func (v *Mp4SampleTableBox) ctts() (*Mp4CompositionTime2SampleBox, error) {
    if box, err := v.get(SrsMp4BoxTypeCTTS); err != nil {
        return nil, err
    } else {
        return box.(*Mp4CompositionTime2SampleBox), nil
    }
}

func (v *Mp4SampleTableBox) stss() (*Mp4SyncSampleBox, error) {
    if box, err := v.get(SrsMp4BoxTypeSTSS); err != nil {
        return nil, err
    } else {
        return box.(*Mp4SyncSampleBox), nil
    }
}

func (v *Mp4SampleTableBox) stsz() (*Mp4SampleSizeBox, error) {
    if box, err := v.get(SrsMp4BoxTypeSTSZ); err != nil {
        return nil, err
    } else {
        return box.(*Mp4SampleSizeBox), nil
    }
}

func (v *Mp4SampleTableBox) stco() (*Mp4ChunkOffsetBox, error) {
    if box, err := v.get(SrsMp4BoxTypeSTCO); err != nil {
        return nil, err
    } else {
        return box.(*Mp4ChunkOffsetBox), nil
    }
}

func (v *Mp4SampleTableBox) stsd() (*Mp4SampleDescritionBox, error) {
    if box, err := v.get(SrsMp4BoxTypeSTSD); err != nil {
        return nil, err
    } else {
        return box.(*Mp4SampleDescritionBox), nil
    }
}

/**
 * 8.5.2 Sample Description Box
 * ISO_IEC_14496-12-base-format-2012.pdf, page 43
 */
type Mp4SampleEntry struct {
    Mp4Box
    Reserved [6]uint8
    DataReferenceIndex uint16
}

func NewMp4SampleEntry() *Mp4SampleEntry {
    v := &Mp4SampleEntry{
        Reserved: [6]uint8{},
    }
    return v
}

func (v *Mp4SampleEntry) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4SampleEntry) DecodeHeader(r io.Reader) (err error) {
    v.Skip(r, uint64(6))
    if err = v.Read(r, &v.DataReferenceIndex); err != nil {
        ol.E(nil, fmt.Sprintf("read sample entry data ref index failed, err is %v", err))
        return
    }
    ol.I(nil, fmt.Sprintf("decode sample entry success, entry:%+v", v))
    return
}

/**
 * 8.5.2 Sample Description Box (avc1)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 44
 */
type Mp4VisualSampleEntry struct {
    Mp4SampleEntry
    PreDefined0     uint16
    Reserved0       uint16
    PreDefined1     [3]uint32
    Width           uint16
    Height          uint16
    HorizResolution uint32
    VertResolution  uint32
    Reserved1       uint32
    FrameCount      uint16
    CompressorName  []uint8
    Depth           uint16
    PreDefined2     int16
}

func NewMp4VisualSampleEntry() *Mp4VisualSampleEntry {
    v := &Mp4VisualSampleEntry{
        PreDefined1: [3]uint32{},
        FrameCount: 1,
        HorizResolution: 0x00480000,
        VertResolution: 0x00480000,
        Depth: 0x0018,
        PreDefined2: -1,
    }
    v.CompressorName = make([]uint8, 32)
    return v
}

func (v *Mp4VisualSampleEntry) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4SampleEntry.DecodeHeader(r); err != nil {
        return
    }

    v.Skip(r, uint64(2))
    v.Skip(r, uint64(2))
    v.Skip(r, uint64(12))

    if err = v.Read(r, &v.Width); err != nil {
        ol.E(nil, fmt.Sprintf("read avc1 width failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.Height); err != nil {
        ol.E(nil, fmt.Sprintf("read avc1 height failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.HorizResolution); err != nil {
        ol.E(nil, fmt.Sprintf("read avc1 horizon resolution failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.VertResolution); err != nil {
        ol.E(nil, fmt.Sprintf("read avc1 vertical resolution failed, err is %v", err))
        return
    }

    v.Skip(r, uint64(4))

    if err = v.Read(r, &v.FrameCount); err != nil {
        ol.E(nil, fmt.Sprintf("read avc1 frame count failed, err is %v", err))
        return
    }
    ol.T(nil, fmt.Sprintf("after read frame count, usedSize=%v", v.UsedSize))

    if err = v.Read(r, v.CompressorName); err != nil {
        ol.E(nil, fmt.Sprintf("read avc1 compressor name failed, err is %v", err))
        return
    }
    ol.T(nil, fmt.Sprintf("after read CompressorName, usedSize=%v", v.UsedSize))

    if err = v.Read(r, &v.Depth); err != nil {
        ol.E(nil, fmt.Sprintf("read avc1 depth failed, err is %v", err))
        return
    }

    v.Skip(r, uint64(2))
    ol.T(nil, fmt.Sprintf("decode avc1 succes, data:%+v, left:%v", v, v.left()))
    return
}

func (v *Mp4VisualSampleEntry) avcc() (*Mp4AvccBox, error) {
    if box, err := v.get(SrsMp4BoxTypeAVCC); err != nil {
        return nil, err
    } else {
        return box.(*Mp4AvccBox), nil
    }
}

/**
 * 5.3.4 AVC Video Stream Definition (avcC)
 * ISO_IEC_14496-15-AVC-format-2012.pdf, page 19
 */
type Mp4AvccBox struct {
    Mp4Box
    nbConfig int
    avcConfig []uint8
}

func (v *Mp4AvccBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4AvccBox) DecodeHeader(r io.Reader) (err error) {
    v.nbConfig = int(v.left())
    v.avcConfig = make([]uint8, v.nbConfig)
    if err = v.Read(r, v.avcConfig); err != nil {
        ol.E(nil, fmt.Sprintf("read avcc config failed, err is %v", err))
        return
    }
    ol.T(nil, fmt.Sprintf("read avcc box success, nv config=%v", v.nbConfig))
    return
}

/**
 * 8.5.2 Sample Description Box (mp4a)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 45
 */
type Mp4AudioSampleEntry struct {
    Mp4SampleEntry
    reserved0 uint64
    channelCount uint16
    sampleSize uint16
    preDefined0 uint16
    reserved1 uint16
    sampleRate uint32
}

func (v *Mp4AudioSampleEntry) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4SampleEntry.DecodeHeader(r); err != nil {
        return
    }

    v.Skip(r, uint64(8))

    if err = v.Read(r, &v.channelCount); err != nil {
        ol.E(nil, fmt.Sprintf("read mp4a channel count failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.sampleSize); err != nil {
        ol.E(nil, fmt.Sprintf("read mp4a sample size failed, err is %v", err))
        return
    }

    v.Skip(r, uint64(2))
    v.Skip(r, uint64(2))

    if err = v.Read(r, &v.sampleRate); err != nil {
        ol.E(nil, fmt.Sprintf("read mp4a sample rate failed, err is %v", err))
        return
    }

    ol.T(nil, fmt.Sprintf("decode mp4a succes, data:%+v %v", v, v.left()))
    return
}

func (v *Mp4AudioSampleEntry) esds() (*Mp4EsdsBox, error) {
    if box, err := v.get(SrsMp4BoxTypeESDS); err != nil {
        return nil, err
    } else {
        return box.(*Mp4EsdsBox), nil
    }
}

func (v *Mp4AudioSampleEntry) asc() (*Mp4DecoderSpecificInfo, error) {
    if box, err := v.esds(); err != nil {
        return nil, err
    } else {
        return box.asc()
    }
}

/**
 * 7.2.2.2 BaseDescriptor
 * ISO_IEC_14496-1-System-2010.pdf, page 32
 */
type Mp4BaseDescriptor struct {
    // The values of the class tags are
    // defined in Table 2. As an expandable class the size of each class instance in bytes is encoded and accessible
    // through the instance variable sizeOfInstance (see 8.3.3).
    tag uint8 // bit(8)
    // The decoded or encoded variant length.
    vlen int32 // bit(28)

    total int32

    usedSize int32
}

func (v *Mp4BaseDescriptor) decodeHeader(r io.Reader) (err error) {
    if err = binary.Read(r, binary.BigEndian, &v.tag); err != nil {
        ol.E(nil, fmt.Sprintf("read desc tag failed, err is %v", err))
        return
    }
    v.total += 1

    var vsize uint8
    var length int32
    for {
        if err = binary.Read(r, binary.BigEndian, &vsize); err != nil {
            ol.E(nil, fmt.Sprintf("read desc 1byte size failed, err is %v", err))
            return
        }
        length = (length << 7) | int32(vsize & 0x7f)
        v.total += 1

        if (vsize & 0x80 ) != 0x80 {
            break
        }
    }
    v.vlen = length
    v.total += length
    return
}

func (v *Mp4BaseDescriptor) Read(r io.Reader, data interface{}) (err error) {
    if err = binary.Read(r, binary.BigEndian, data); err != nil {
        return
    }
    v.usedSize += int32(uint64DataSize(data))
    return
}

func (v *Mp4BaseDescriptor) totalSize() int32 {
    return v.total
}

func (v *Mp4BaseDescriptor) left() int32 {
    return v.vlen - v.usedSize
}

/**
 * 7.2.6.7 DecoderSpecificInfo
 * ISO_IEC_14496-1-System-2010.pdf, page 51
 */
type Mp4DecoderSpecificInfo struct {
    Mp4BaseDescriptor
    asc []uint8
}

func NewMp4DecoderSpecificInfo() *Mp4DecoderSpecificInfo {
    v := &Mp4DecoderSpecificInfo{
        asc: []uint8{},
    }
    return v
}

func (v *Mp4DecoderSpecificInfo) decode(r io.Reader) (err error) {
    if err = v.Mp4BaseDescriptor.decodeHeader(r); err != nil {
        return
    }

    v.asc = make([]uint8, v.vlen)
    if err = v.Read(r, v.asc); err != nil {
        ol.E(nil, fmt.Sprintf("read DecoderSpecificInfo asc failed, err is %v", err))
        return
    }

    ol.T(nil, fmt.Sprintf("decode specificInfo:asc:%+v", v.asc))
    return
}

/**
 * 7.2.6.6 DecoderConfigDescriptor
 * ISO_IEC_14496-1-System-2010.pdf, page 48
 */
type Mp4DecoderConfigDescriptor struct {
    Mp4BaseDescriptor

    // an indication of the object or scene description type that needs to be supported
    // by the decoder for this elementary stream as per Table 5.
    objectTypeIndication uint8 // bit(8)
    streamType uint8 // bit(6)
    upStream uint8 // bit(1)
    reserved uint8 // bit(1)
    bufferSizeDB uint32 // bit(24)
    maxBitrate uint32
    avgBitrate uint32
    descSpecificInfo *Mp4DecoderSpecificInfo // optional.
}

func NewMp4DecoderConfigDescriptor() *Mp4DecoderConfigDescriptor {
    v := &Mp4DecoderConfigDescriptor {
        descSpecificInfo: &Mp4DecoderSpecificInfo{},
    }
    return v
}

func (v *Mp4DecoderConfigDescriptor) decode(r io.Reader) (err error) {
    if err = v.Mp4BaseDescriptor.decodeHeader(r); err != nil {
        return
    }

    if err = v.Read(r, &v.objectTypeIndication); err != nil {
        ol.E(nil, fmt.Sprintf("read DecoderConfigDescriptor objectTypeIndication failed, err is %v", err))
        return
    }

    var data uint8
    if err = v.Read(r, &data); err != nil {
        ol.E(nil, fmt.Sprintf("read DecoderConfigDescriptor data failed, err is %v", err))
        return
    }
    v.upStream = (data >> 1) & 0x01
    v.streamType = (data >> 2) & 0x3f
    v.reserved = data & 0x01

    tmp := make([]byte, 3)
    if _, err = io.ReadFull(r, tmp); err != nil {
        ol.E(nil, fmt.Sprintf("read DecoderConfigDescriptor bufferSizeDB failed, err is %v", err))
        return
    }
    v.bufferSizeDB = Bytes3ToUint32(tmp)

    if err = v.Read(r, &v.maxBitrate); err != nil {
        ol.E(nil, fmt.Sprintf("read DecoderConfigDescriptor maxBitrate failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.avgBitrate); err != nil {
        ol.E(nil, fmt.Sprintf("read DecoderConfigDescriptor avgBitrate failed, err is %v", err))
        return
    }

    ol.T(nil, fmt.Sprintf("after decode DecoderConfigDescriptor, left:%v", v.left()))
    if v.left() > 0 {
        if err = v.descSpecificInfo.decode(r); err != nil {
            ol.E(nil, fmt.Sprintf("decode descSpecificInfo failed, err is %v", err))
            return
        }
    }

    ol.T(nil, fmt.Sprintf("decode config desc:%+v", v))
    return
}

/**
 * 7.3.2.3 SL Packet Header Configuration
 * ISO_IEC_14496-1-System-2010.pdf, page 92
 */
type Mp4SLConfigDescriptor struct {
    Mp4BaseDescriptor
    predefined uint8
}

func (v *Mp4SLConfigDescriptor) decode(r io.Reader) (err error) {
    if err = v.Mp4BaseDescriptor.decodeHeader(r); err != nil {
        return
    }

    if err = v.Read(r, &v.predefined); err != nil {
        ol.E(nil, fmt.Sprintf("read SL predefined failed, err is %v", err))
        return
    }
    ol.T(nil, fmt.Sprintf("decde sl:predefined:%v", v.predefined))
    return
}

/**
 * 7.2.6.5 ES_Descriptor
 * ISO_IEC_14496-1-System-2010.pdf, page 47
 */
type Mp4ES_Descriptor struct {
    Mp4BaseDescriptor

    ES_ID uint16
    streamDependenceFlag uint8 // bit(1)
    URL_Flag uint8 // bit(1)
    OCRstreamFlag uint8 // bit(1)
    streamPriority uint8 // bit(5)
                               // if (streamDependenceFlag)
    dependsOn_ES_ID uint16
                               // if (URL_Flag)
    URLlength uint8
    URLstring []uint8
                               // if (OCRstreamFlag)
    OCR_ES_Id uint16

    decConfigDescr *Mp4DecoderConfigDescriptor
    slConfigDescr *Mp4SLConfigDescriptor
}

func NewMp4ES_Descriptor() *Mp4ES_Descriptor {
    v := &Mp4ES_Descriptor{
        decConfigDescr: NewMp4DecoderConfigDescriptor(),
        slConfigDescr: &Mp4SLConfigDescriptor{},
    }
    return v
}

func (v *Mp4ES_Descriptor) decode(r io.Reader) (err error) {
    if err = v.Mp4BaseDescriptor.decodeHeader(r); err != nil {
        return
    }

    if err = v.Read(r, &v.ES_ID); err != nil {
        ol.E(nil, fmt.Sprintf("read ES_Descriptor ES_ID failed, err is %v", err))
        return
    }

    var data uint8
    if err = v.Read(r, &data); err != nil {
        ol.E(nil, fmt.Sprintf("read ES_Descriptor data failed, err is %v", err))
        return
    }
    v.streamPriority = data & 0x1f
    v.streamDependenceFlag = (data >> 7) & 0x01
    v.URL_Flag = (data >> 6) & 0x01
    v.OCRstreamFlag = (data >> 5) & 0x01

    if v.streamDependenceFlag == 0x01 {
        if err = v.Read(r, &v.dependsOn_ES_ID); err != nil {
            ol.E(nil, fmt.Sprintf("read ES_Descriptor dependsOn_ES_ID failed, err is %v", err))
            return
        }
    }

    if v.URL_Flag == 0x01 {
        if err = v.Read(r, &v.URLlength); err != nil {
            ol.E(nil, fmt.Sprintf("read ES_Descriptor URLLength failed, err is %v", err))
            return
        }

        v.URLstring = make([]uint8, v.URLlength)
        if err = v.Read(r, v.URLstring); err != nil {
            ol.E(nil, fmt.Sprintf("read ES_Descriptor URLstring failed, err is %v", err))
            return
        }
    }

    if v.OCRstreamFlag == 0x01 {
        if err = v.Read(r, &v.OCR_ES_Id); err != nil {
            ol.E(nil, fmt.Sprintf("read ES_Descriptor OCR_ES_Id failed, err is %v", err))
            return
        }
    }

    if err = v.decConfigDescr.decode(r); err != nil {
        ol.E(nil, fmt.Sprintf("decode ES_Descriptor decConfigDescr failed, err is %v", err))
        return
    }
    if err = v.slConfigDescr.decode(r); err != nil {
        ol.E(nil, fmt.Sprintf("decode ES_Descriptor slConfigDescr failed, err is %v", err))
        return
    }

    ol.T(nil, fmt.Sprintf("decode ES_Descriptor:%+v", v))
    return
}

/**
 * 5.6 Sample Description Boxes
 * Elementary Stream Descriptors (esds)
 * ISO_IEC_14496-14-MP4-2003.pdf, page 15
 * @see http://www.mp4ra.org/codecs.html
 */
type Mp4EsdsBox struct {
    Mp4FullBox
    es *Mp4ES_Descriptor
}

func  NewMp4EsdsBox() *Mp4EsdsBox {
    v := &Mp4EsdsBox{
        es: NewMp4ES_Descriptor(),
    }
    return v
}

func (v *Mp4EsdsBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4EsdsBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if err = v.es.decode(r); err != nil {
        ol.E(nil, fmt.Sprintf("decode esds box failed, err is %v", err))
    }
    ol.T(nil, fmt.Sprintf("before decode esds content, used=%v es_len=%v", v.UsedSize, v.es.total))

    v.UsedSize += uint64(v.es.total)
    return
}

func (v *Mp4EsdsBox) asc() (*Mp4DecoderSpecificInfo, error) {
    return v.es.decConfigDescr.descSpecificInfo, nil
}

/**
 * 8.5.2 Sample Description Box (stsd), for Audio/Video.
 * ISO_IEC_14496-12-base-format-2012.pdf, page 40
 * The sample description table gives detailed information about the coding type used, and any initialization
 * information needed for that coding.
 */
type Mp4SampleDescritionBox struct {
    Mp4FullBox
    Entries []Box
}

func NewMp4SampleDescritionBox() *Mp4SampleDescritionBox {
    v := &Mp4SampleDescritionBox{
        Entries: []Box{},
    }
    return v
}

func (v *Mp4SampleDescritionBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4SampleDescritionBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    var nbEntries uint32
    if err = v.Read(r, &nbEntries); err != nil {
        ol.E(nil, fmt.Sprintf("read stsd number entries failed, err is %v", err))
        return
    }

    for i := 0; i < int(nbEntries); i++ {
        mb := NewMp4Box()
        var subBox Box
        if subBox, err = mb.discovery(r); err != nil {
            return
        }

        if err = subBox.DecodeHeader(r); err != nil {
            return
        }

        if err = subBox.Basic().DecodeBoxes(r); err != nil {
            return
        }

        v.Entries = append(v.Entries, subBox)
        v.UsedSize += subBox.Basic().sz()

        ol.T(nil, fmt.Sprintf("decode one entry, box:%v, basic.sz=%v, usedSize=%v, left=%v", reflect.TypeOf(subBox), subBox.Basic().sz(), v.UsedSize, v.left()))
    }

    ol.T(nil, fmt.Sprintf("decode stsd box success, box:%+v", v))
    return
}

func (v *Mp4SampleDescritionBox) mp4a() (*Mp4AudioSampleEntry, error) {
    for _, entry := range v.Entries {
        if et, ok := entry.(*Mp4AudioSampleEntry); ok {
            return et, nil
        }
    }
    return nil, fmt.Errorf("can't find mp4a in stsd")
}

func (v *Mp4SampleDescritionBox) avc1() (*Mp4VisualSampleEntry, error) {
    for _, entry := range v.Entries {
        if et, ok := entry.(*Mp4VisualSampleEntry); ok {
            return et, nil
        }
    }
    return nil, fmt.Errorf("can't find avc1 in stsd")
}

/**
 * 8.6.1.2 Decoding Time to Sample Box (stts), for Audio/Video.
 * ISO_IEC_14496-12-base-format-2012.pdf, page 48
 */
type Mp4SttsEntry struct {
    // an integer that counts the number of consecutive samples that have the given
    // duration.
    sampleCount uint32
    // an integer that gives the delta of these samples in the time-scale of the media.
    sampleDelta uint32
}

/**
 * 8.6.1.2 Decoding Time to Sample Box (stts), for Audio/Video.
 * ISO_IEC_14496-12-base-format-2012.pdf, page 48
 * This box contains a compact version of a table that allows indexing from decoding time to sample number.
 * Other tables give sample sizes and pointers, from the sample number. Each entry in the table gives the
 * number of consecutive samples with the same time delta, and the delta of those samples. By adding the
 * deltas a complete time-to-sample map may be built.
 */
type Mp4DecodingTime2SampleBox struct {
    Mp4FullBox
    // an integer that gives the number of entries in the following table.
    entryCount uint32
    entries    []*Mp4SttsEntry

    index      uint32
    count      uint32
}

func NewMp4DecodingTime2SampleBox() *Mp4DecodingTime2SampleBox {
    v := &Mp4DecodingTime2SampleBox{
        entries: []*Mp4SttsEntry{},
    }
    return v
}

func (v *Mp4DecodingTime2SampleBox) initialize_counter() (err error) {
    v.index = 0
    if len(v.entries) == 0 {
        return fmt.Errorf("MP4 illegal ts, empty stts")
    }

    v.count = v.entries[0].sampleCount
    return
}

func (v *Mp4DecodingTime2SampleBox) on_sample(sampleIndex uint32) (entry *Mp4SttsEntry, err error) {
    if (sampleIndex + 1) > v.count {
        v.index ++
        if v.index >= v.entryCount {
            err = fmt.Errorf("MP4 illegal ts, stts overflow, count=%d, index=%v", v.entryCount, v.index)
            return
        }
        v.count += v.entries[v.index].sampleCount
    }

    ol.I(nil, fmt.Sprintf("stts on sample, index=%v, entries=%v", v.index, len(v.entries)))
    entry = v.entries[v.index]

    return
}

func (v *Mp4DecodingTime2SampleBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if err = v.Read(r, &v.entryCount); err != nil {
        ol.E(nil, fmt.Sprintf("read stts entry count failed, err is %v", err))
        return
    }

    for i := 0; i < int(v.entryCount); i++ {
        entry := &Mp4SttsEntry{}
        if err = v.Read(r, &entry.sampleCount); err != nil {
            ol.E(nil, fmt.Sprintf("read stts entry sample count failed, err is %v", err))
            return
        }
        if err = v.Read(r, &entry.sampleDelta); err != nil {
            ol.E(nil, fmt.Sprintf("read stts entry sample delta failed, err is %v", err))
            return
        }
        ol.I(nil, fmt.Sprintf("decode one stts entry, entry=%+v", entry))
        v.entries = append(v.entries, entry)
    }

    ol.I(nil, fmt.Sprintf("decode stts box success, box=%+v", v))
    return
}

func (v *Mp4DecodingTime2SampleBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

/**
 * 8.6.1.3 Composition Time to Sample Box (ctts), for Video.
 * ISO_IEC_14496-12-base-format-2012.pdf, page 49
 */
type Mp4CttsEntry struct {
    // an integer that counts the number of consecutive samples that have the given offset.
    sampleCount uint32
    // uint32_t for version=0
    // int32_t for version=1
    // an integer that gives the offset between CT and DT, such that CT(n) = DT(n) +
    // CTTS(n).
    sampleOffset int64
}

/**
* 8.6.1.3 Composition Time to Sample Box (ctts), for Video.
* ISO_IEC_14496-12-base-format-2012.pdf, page 49
* This box provides the offset between decoding time and composition time. In version 0 of this box the
* decoding time must be less than the composition time, and the offsets are expressed as unsigned numbers
* such that CT(n) = DT(n) + CTTS(n) where CTTS(n) is the (uncompressed) table entry for sample n. In version
* 1 of this box, the composition timeline and the decoding timeline are still derived from each other, but the
* offsets are signed. It is recommended that for the computed composition timestamps, there is exactly one with
* the value 0 (zero).
*/
type Mp4CompositionTime2SampleBox struct {
    Mp4FullBox
    entryCount uint32
    entries []*Mp4CttsEntry

    index uint32
    count uint32
}

func NewMp4CompositionTime2SampleBox() *Mp4CompositionTime2SampleBox {
    v := &Mp4CompositionTime2SampleBox{
        entries: []*Mp4CttsEntry{},
    }
    return v
}

func (v *Mp4CompositionTime2SampleBox) initialize_counter() (err error) {
    v.index = 0
    if len(v.entries) == 0 {
        return fmt.Errorf("MP4 illegal ts, empty ctts")
    }

    v.count = v.entries[0].sampleCount
    return
}

func (v *Mp4CompositionTime2SampleBox) on_sample(sampleIndex uint32) (entry *Mp4CttsEntry, err error) {
    if (sampleIndex + 1) > v.count {
        v.index ++
        if v.index >= v.entryCount {
            err = fmt.Errorf("MP4 illegal ts, stts overflow, count=%d, req sample_index=%v", v.entryCount, v.index)
            return
        }
        v.count += v.entries[v.index].sampleCount
    }

    entry = v.entries[v.index]

    return
}

func (v *Mp4CompositionTime2SampleBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if err = v.Read(r, &v.entryCount); err != nil {
        ol.E(nil, fmt.Sprintf("read stts entry count failed, err is %v", err))
        return
    }

    for i := 0; i < int(v.entryCount); i++ {
        entry := &Mp4CttsEntry{}
        if err = v.Read(r, &entry.sampleCount); err != nil {
            ol.E(nil, fmt.Sprintf("read ctts entry sample count failed, err is %v", err))
            return
        }
        if v.Version == 0 {
            var offset uint32
            v.Read(r, &offset)
            entry.sampleOffset = int64(offset)
        } else if v.Version == 1 {
            var offset int32
            v.Read(r, &offset)
            entry.sampleOffset = int64(offset)
        }
        ol.I(nil, fmt.Sprintf("decode one ctts entry, entry=%+v", entry))
        v.entries = append(v.entries, entry)
    }

    ol.I(nil, fmt.Sprintf("decode vtts box success, box=%+v", v))
    return

    return
}

func (v *Mp4CompositionTime2SampleBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

/**
 * 8.6.2 Sync Sample Box (stss), for Video.
 * ISO_IEC_14496-12-base-format-2012.pdf, page 51
 * This box provides a compact marking of the sync samples within the stream. The table is arranged in strictly
 * increasing order of sample number.
 */
type Mp4SyncSampleBox struct {
    Mp4FullBox
    // an integer that gives the number of entries in the following table. If entry_count is zero,
    // there are no sync samples within the stream and the following table is empty.
    EntryCount uint32
    // the numbers of the samples that are sync samples in the stream.
    SampleNumbers []uint32
}

func NewMp4SyncSampleBox() *Mp4SyncSampleBox {
    v := &Mp4SyncSampleBox{
        SampleNumbers: []uint32{},
    }
    return v
}

func (v *Mp4SyncSampleBox) isSync(index uint32) bool {
    var i uint32
    for i = 0; i < v.EntryCount; i++ {
        if (index + 1) == v.SampleNumbers[i] {
            return true
        }
    }
    return false
}

func (v *Mp4SyncSampleBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if err = v.Read(r, &v.EntryCount); err != nil {
        ol.E(nil, fmt.Sprintf("read stss entry count failed, err is %v", err))
        return
    }

    for i := 0; i < int(v.EntryCount); i++ {
        var sm uint32
        if err = v.Read(r, &sm); err != nil {
            ol.I(nil, fmt.Sprintf("read stss entry %v sample number failed, err is %v", i, err))
            return
        }
        v.SampleNumbers = append(v.SampleNumbers, sm)
    }

    ol.I(nil, fmt.Sprintf("decode stss box success, box=%+v", v))
    return
}

func (v *Mp4SyncSampleBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

/**
 * 8.7.4 Sample To Chunk Box (stsc), for Audio/Video.
 * ISO_IEC_14496-12-base-format-2012.pdf, page 58
 */
type Mp4StscEntry struct {
    // an integer that gives the index of the first chunk in this run of chunks that share the
    // same samples-per-chunk and sample-description-index; the index of the first chunk in a track has the
    // value 1 (the first_chunk field in the first record of this box has the value 1, identifying that the first
    // sample maps to the first chunk).
    FirstChunk uint32
    // an integer that gives the number of samples in each of these chunks
    SamplesPerChunk uint32
    // an integer that gives the index of the sample entry that describes the
    // samples in this chunk. The index ranges from 1 to the number of sample entries in the Sample
    // Description Box
    sampleDescriptionIndex uint32
}

/**
 * 8.7.4 Sample To Chunk Box (stsc), for Audio/Video.
 * ISO_IEC_14496-12-base-format-2012.pdf, page 58
 * Samples within the media data are grouped into chunks. Chunks can be of different sizes, and the samples
 * within a chunk can have different sizes. This table can be used to find the chunk that contains a sample,
 * its position, and the associated sample description.
 */
type Mp4Sample2ChunkBox struct {
    Mp4FullBox
    // an integer that gives the number of entries in the following table
    EntryCount uint32
    // the numbers of the samples that are sync samples in the stream.
    Entries []*Mp4StscEntry

    index uint32
}

func NewMp4Sample2ChunkBox() *Mp4Sample2ChunkBox {
    v := &Mp4Sample2ChunkBox{
        Entries: []*Mp4StscEntry{},
    }
    return v
}

func (v *Mp4Sample2ChunkBox) initialize_counter() {
    v.index = 0
}

func (v *Mp4Sample2ChunkBox) onChunk(chunkIndex uint32) (entry *Mp4StscEntry) {
    // Last chunk?
    if v.index >= v.EntryCount - 1 {
        ol.T(nil, fmt.Sprintf("last chunk, stsc.index=%v", v.index))
        return v.Entries[v.index]
    }

    // Move next chunk?
    if (chunkIndex + 1 >= v.Entries[v.index + 1].FirstChunk) {
        ol.T(nil, fmt.Sprintf("move to next chunk"))
        v.index++;
    }
    return v.Entries[v.index]
}

func (v *Mp4Sample2ChunkBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if err = v.Read(r, &v.EntryCount); err != nil {
        ol.E(nil, fmt.Sprintf("read stsc entry count failed, err is %v", err))
        return
    }

    for i := 0; i < int(v.EntryCount); i++ {
        entry := &Mp4StscEntry{}
        if err = v.Read(r, &entry.FirstChunk); err != nil {
            ol.E(nil, fmt.Sprintf("read stsc %v entry first chunk failed, err is %v", i ,err))
            return
        }
        if err = v.Read(r, &entry.SamplesPerChunk); err != nil {
            ol.E(nil, fmt.Sprintf("read stsc %v entry samples per chunk failed, err is %v", i ,err))
            return
        }
        if err = v.Read(r, &entry.sampleDescriptionIndex); err != nil {
            ol.E(nil, fmt.Sprintf("read stsc %v entry samples description index failed, err is %v", i ,err))
            return
        }
        ol.T(nil, fmt.Sprintf("decode stsc entry ok, entry=%+v", entry))
        v.Entries = append(v.Entries, entry)
    }

    ol.I(nil, fmt.Sprintf("decode stsc box success, box=%+v", v))
    return
}

func (v *Mp4Sample2ChunkBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

/**
 * 8.7.3.2 Sample Size Box (stsz), for Audio/Video.
 * ISO_IEC_14496-12-base-format-2012.pdf, page 58
 * This box contains the sample count and a table giving the size in bytes of each sample. This allows the media data
 * itself to be unframed. The total number of samples in the media is always indicated in the sample count.
 */
type Mp4SampleSizeBox struct {
    Mp4FullBox
    // the default sample size. If all the samples are the same size, this field
    // contains that size value. If this field is set to 0, then the samples have different sizes, and those sizes
    // are stored in the sample size table. If this field is not 0, it specifies the constant sample size, and no
    // array follows.
    sampleSize  uint32
    // an integer that gives the number of samples in the track; if sample-size is 0, then it is
    // also the number of entries in the following table.
    sampleCount uint32
    // each entry_size is an integer specifying the size of a sample, indexed by its number.
    entrySizes  []uint32
}

func NewMp4SampleSizeBox() *Mp4SampleSizeBox {
    v := &Mp4SampleSizeBox{
        entrySizes: []uint32{},
    }
    return v
}

func (v *Mp4SampleSizeBox) getSampleSize(index uint32) (size uint32, err error) {
    if v.sampleSize != 0 {
        size = v.sampleSize
        return
    }

    if index >= v.sampleCount {
        err = fmt.Errorf("MP4 stsz overflow, sample_count=%v, req index=%v", v.sampleCount, index)
        return
    }

    size = v.entrySizes[index]
    return
}

func (v *Mp4SampleSizeBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if err = v.Read(r, &v.sampleSize); err != nil {
        ol.E(nil ,fmt.Sprintf("read stsz sample size failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.sampleCount); err!= nil {
        ol.E(nil, fmt.Sprintf("read stsz sample count failed, err is %v", err))
        return
    }

    if v.sampleSize == 0 {
        for i := 0; i < int(v.sampleCount); i++ {
            var size uint32
            if err = v.Read(r, &size); err != nil {
                ol.E(nil, fmt.Sprintf("read stsz %v entry size failed, err is %v", i, err))
                return
            }
            v.entrySizes = append(v.entrySizes, size)
        }
    }

    ol.I(nil, fmt.Sprintf("decode stsz box success, box=%+v", v))
    return
}

func (v *Mp4SampleSizeBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

/**
 * 8.7.5 Chunk Offset Box (stco), for Audio/Video.
 * ISO_IEC_14496-12-base-format-2012.pdf, page 59
 * The chunk offset table gives the index of each chunk into the containing file. There are two variants, permitting
 * the use of 32-bit or 64-bit offsets. The latter is useful when managing very large presentations. At most one of
 * these variants will occur in any single instance of a sample table.
 */
type Mp4ChunkOffsetBox struct {
    Mp4FullBox
    // an integer that gives the number of entries in the following table
    EntryCount uint32
    // a 32 bit integer that gives the offset of the start of a chunk into its containing
    // media file.
    Entries []uint32
}

func NewMp4ChunkOffsetBox() *Mp4ChunkOffsetBox {
    v := &Mp4ChunkOffsetBox{
        Entries: []uint32{},
    }
    return v
}

func (v *Mp4ChunkOffsetBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if err = v.Read(r, &v.EntryCount); err != nil {
        ol.E(nil, fmt.Sprintf("read stco entry count failed, err is %v", err))
        return
    }

    for i := 0; i < int(v.EntryCount); i++ {
        var entry uint32
        if err = v.Read(r, &entry); err != nil {
            ol.E(nil, fmt.Sprintf("read stco %v entry failed, err is %v", i, err))
            return
        }
        v.Entries = append(v.Entries, entry)
    }

    ol.I(nil, fmt.Sprintf("decode stco box success, box=%+v", v))
    return
}

func (v *Mp4ChunkOffsetBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

/**
 * 8.10.1 User Data Box (udta)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 78
 * This box contains objects that declare user information about the containing box and its data (presentation or
 * track).
 */
type Mp4UserDataBox struct {
    Mp4Box
    NbData int
    Data []uint8
}

func NewMp4UserDataBox() *Mp4UserDataBox {
    v := &Mp4UserDataBox{
        Data: []uint8{},
    }
    return v
}

func (v *Mp4UserDataBox) DecodeHeader(r io.Reader) (err error) {
    v.NbData = int(v.left())
    v.Skip(r, v.left())
    ol.I(nil, fmt.Sprintf("decode udta box success, nb data=%v", v.NbData))
    return
}

func (v *Mp4UserDataBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

/**
 * 8.1.1 Media Data Box (mdat)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 29
 * This box contains the media data. In video tracks, this box would contain video frames.
 * A presentation may contain zero or more Media Data Boxes. The actual media data follows the type field;
 * its structure is described by the metadata (see particularly the sample table, subclause 8.5, and the
 * item location box, subclause 8.11.3).
 */
type Mp4MediaDataBox struct {
    Mp4Box
    NbData int
    Data []uint8
}

func NewMp4MediaDataBox() *Mp4MediaDataBox {
    v:= &Mp4MediaDataBox{
        Data: []uint8{},
    }
    return v
}

func (v *Mp4MediaDataBox) DecodeHeader(r io.Reader) (err error) {
    v.NbData = int(v.left())
    v.Skip(r, v.left())
    ol.I(nil, fmt.Sprintf("decode mdat box success, nb data=%v", v.NbData))
    return
}

func (v *Mp4MediaDataBox) Basic() *Mp4Box {
    return &v.Mp4Box
}






