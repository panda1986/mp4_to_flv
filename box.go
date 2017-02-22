package main

import (
    "fmt"
    "io"
    ol "github.com/ossrs/go-oryx-lib/logger"
    "encoding/binary"
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

// Box type helper.
func (v *Mp4Box) IsFtyp() bool {
    return v.BoxType == SrsMp4BoxTypeFTYP
}
func (v *Mp4Box) IsMoov() bool {
    return v.BoxType == SrsMp4BoxTypeMOOV
}
func (v *Mp4Box) IsMdat() bool {
    return v.BoxType == SrsMp4BoxTypeMDAT
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
func (v *Mp4Box) get(bt uint32) (error, Box) {
    for _, box := range v.Boxes {
        if box.Basic().BoxType == bt {
            return nil, box
        }
    }
    return fmt.Errorf("can't find bt:%v in boxes", bt), nil
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

    ol.I(nil, fmt.Sprintf("discovery a new box small size=%v, large size=%v, bt=%x", smallSize, largeSize, bt))
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
    case SrsMp4BoxTypeSTSD:
        box = NewMp4SampleDescritionBox()
    case SrsMp4BoxTypeSTTS:
        box = NewMp4DecodingTime2SampleBox()
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
        v.Boxes = append(v.Boxes, box)

        left -= box.Basic().sz()
    }
    return
}

func (v *Mp4Box) Skip(r io.Reader, num uint64) {
    data := make([]uint8, num)
    v.Read(r, data)
    v.UsedSize += num
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
func (v *Mp4MovieBox) Mvhd() *Mp4MovieHeaderBox {
    return nil
}

func (v *Mp4MovieBox) AddTrack() {

}

// Get the number of video tracks
func (v *Mp4MovieBox) NbVideoTracks() int {
    return 0
}

// Get the number of audio tracks
func (v *Mp4MovieBox) NbSoundTracks() int {
    return 0
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
    TrackType uint8
}

func (v *Mp4TrackBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4TrackBox) NdHeader() int {
    return v.Mp4Box.NbHeader()
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
    CompressorName  [32]uint8
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

    if err = v.Read(r, &v.CompressorName); err != nil {
        ol.E(nil, fmt.Sprintf("read avc1 compressor name failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.Depth); err != nil {
        ol.E(nil, fmt.Sprintf("read avc1 depth failed, err is %v", err))
        return
    }

    v.Skip(r, uint64(2))

    ol.I(nil, fmt.Sprintf("decode avc1 succes, data:%+v", v))
    return
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
        var box Box
        if box, err = mb.discovery(r); err != nil {
            return
        }

        if err = box.DecodeHeader(r); err != nil {
            return
        }

        v.Entries = append(v.Entries, box)
        v.UsedSize += box.Basic().sz()

        ol.I(nil, fmt.Sprintf("decode one entry, basic.sz=%v, usedSize=%v, left=%v", box.Basic().sz(), v.UsedSize, v.left()))
    }

    ol.I(nil, fmt.Sprintf("decode stsd box success, box:%+v", v))
    return
}

/**
 * 8.6.1.2 Decoding Time to Sample Box (stts), for Audio/Video.
 * ISO_IEC_14496-12-base-format-2012.pdf, page 48
 */
type Mp4SttsEntry struct {
    // an integer that counts the number of consecutive samples that have the given
    // duration.
    SampleCount uint32
    // an integer that gives the delta of these samples in the time-scale of the media.
    SampleDelta uint32
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
    EntryCount uint32
    Entries []*Mp4SttsEntry
}

func NewMp4DecodingTime2SampleBox() *Mp4DecodingTime2SampleBox {
    v := &Mp4DecodingTime2SampleBox{
        Entries: []*Mp4SttsEntry{},
    }
    return v
}

func (v *Mp4DecodingTime2SampleBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if err = v.Read(r, &v.EntryCount); err != nil {
        ol.E(nil, fmt.Sprintf("read stts entry count failed, err is %v", err))
        return
    }

    for i := 0; i < int(v.EntryCount); i++ {
        entry := &Mp4SttsEntry{}
        if err = v.Read(r, &entry.SampleCount); err != nil {
            ol.E(nil, fmt.Sprintf("read stts entry sample count failed, err is %v", err))
            return
        }
        if err = v.Read(r, &entry.SampleDelta); err != nil {
            ol.E(nil, fmt.Sprintf("read stts entry sample delta failed, err is %v", err))
            return
        }
        ol.I(nil, fmt.Sprintf("decode one stts entry, entry=%+v", entry))
        v.Entries = append(v.Entries, entry)
    }

    ol.I(nil, fmt.Sprintf("decode stts box success, box=%+v", v))
    return
}

func (v *Mp4DecodingTime2SampleBox) Basic() *Mp4Box {
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
    FirstChunk uint32
    SamplesPerChunk uint32
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
}

func NewMp4Sample2ChunkBox() *Mp4Sample2ChunkBox {
    v := &Mp4Sample2ChunkBox{
        Entries: []*Mp4StscEntry{},
    }
    return v
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
        ol.I(nil, fmt.Sprintf("decode stsc entry ok, entry=%+v", entry))
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
    SampleSize uint32
    // an integer that gives the number of samples in the track; if sample-size is 0, then it is
    // also the number of entries in the following table.
    SampleCount uint32
    // each entry_size is an integer specifying the size of a sample, indexed by its number.
    EntrySizes []uint32
}

func NewMp4SampleSizeBox() *Mp4SampleSizeBox {
    v := &Mp4SampleSizeBox{
        EntrySizes: []uint32{},
    }
    return v
}

func (v *Mp4SampleSizeBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if err = v.Read(r, &v.SampleSize); err != nil {
        ol.E(nil ,fmt.Sprintf("read stsz sample size failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.SampleCount); err!= nil {
        ol.E(nil, fmt.Sprintf("read stsz sample count failed, err is %v", err))
        return
    }

    if v.SampleSize == 0 {
        for i := 0; i < int(v.SampleCount); i++ {
            var size uint32
            if err = v.Read(r, &size); err != nil {
                ol.E(nil, fmt.Sprintf("read stsz %v entry size failed, err is %v", i, err))
                return
            }
            v.EntrySizes = append(v.EntrySizes, size)
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






