package main

const (
    SRS_MP4_EOF_SIZE = 0
    SRS_MP4_USE_LARGE_SIZE = 1

    SrsMp4BoxTypeForbidden = 0x00

    SrsMp4BoxTypeUUID = 0x75756964 // 'uuid'
    SrsMp4BoxTypeFTYP = 0x66747970 // 'ftyp'
    SrsMp4BoxTypeMDAT = 0x6d646174 // 'mdat'
    SrsMp4BoxTypeFREE = 0x66726565 // 'free'
    SrsMp4BoxTypeSKIP = 0x736b6970 // 'skip'
    SrsMp4BoxTypeMOOV = 0x6d6f6f76 // 'moov'
    SrsMp4BoxTypeMVHD = 0x6d766864 // 'mvhd'
    SrsMp4BoxTypeTRAK = 0x7472616b // 'trak'
    SrsMp4BoxTypeTKHD = 0x746b6864 // 'tkhd'
    SrsMp4BoxTypeEDTS = 0x65647473 // 'edts'
    SrsMp4BoxTypeELST = 0x656c7374 // 'elst'
    SrsMp4BoxTypeMDIA = 0x6d646961 // 'mdia'
    SrsMp4BoxTypeMDHD = 0x6d646864 // 'mdhd'
    SrsMp4BoxTypeHDLR = 0x68646c72 // 'hdlr'
    SrsMp4BoxTypeMINF = 0x6d696e66 // 'minf'
    SrsMp4BoxTypeVMHD = 0x766d6864 // 'vmhd'
    SrsMp4BoxTypeSMHD = 0x736d6864 // 'smhd'
    SrsMp4BoxTypeDINF = 0x64696e66 // 'dinf'
    SrsMp4BoxTypeURL  = 0x75726c20 // 'url '
    SrsMp4BoxTypeURN  = 0x75726e20 // 'urn '
    SrsMp4BoxTypeDREF = 0x64726566 // 'dref'
    SrsMp4BoxTypeSTBL = 0x7374626c // 'stbl'
    SrsMp4BoxTypeSTSD = 0x73747364 // 'stsd'
    SrsMp4BoxTypeSTTS = 0x73747473 // 'stts'
    SrsMp4BoxTypeCTTS = 0x63747473 // 'ctts'
    SrsMp4BoxTypeSTSS = 0x73747373 // 'stss'
    SrsMp4BoxTypeSTSC = 0x73747363 // 'stsc'
    SrsMp4BoxTypeSTCO = 0x7374636f // 'stco'
    SrsMp4BoxTypeCO64 = 0x636f3634 // 'co64'
    SrsMp4BoxTypeSTSZ = 0x7374737a // 'stsz'
    SrsMp4BoxTypeSTZ2 = 0x73747a32 // 'stz2'
    SrsMp4BoxTypeAVC1 = 0x61766331 // 'avc1'
    SrsMp4BoxTypeAVCC = 0x61766343 // 'avcC'
    SrsMp4BoxTypeMP4A = 0x6d703461 // 'mp4a'
    SrsMp4BoxTypeESDS = 0x65736473 // 'esds'
    SrsMp4BoxTypeUDTA = 0x75647461 // 'udta'

    SrsMp4BoxBrandForbidden = 0x00
    SrsMp4BoxBrandISOM = 0x69736f6d // 'isom'
    SrsMp4BoxBrandISO2 = 0x69736f32 // 'iso2'
    SrsMp4BoxBrandAVC1 = 0x61766331 // 'avc1'
    SrsMp4BoxBrandMP41 = 0x6d703431 // 'mp41'

    // The type of track, maybe combine of types.
    SrsMp4TrackTypeForbidden = 0x00
    SrsMp4TrackTypeAudio = 0x01
    SrsMp4TrackTypeVideo = 0x02
)

/**
 * The video codec id.
 * @doc video_file_format_spec_v10_1.pdf, page78, E.4.3.1 VIDEODATA
 * CodecID UB [4]
 * Codec Identifier. The following values are defined for FLV:
 *      2 = Sorenson H.263
 *      3 = Screen video
 *      4 = On2 VP6
 *      5 = On2 VP6 with alpha channel
 *      6 = Screen video version 2
 *      7 = AVC
 */
const (
    // set to the zero to reserved, for array map.
    SrsVideoCodecIdReserved = 0
    SrsVideoCodecIdForbidden = 0
    SrsVideoCodecIdReserved1 = 1
    SrsVideoCodecIdReserved2 = 9

    // for user to disable video, for example, use pure audio hls.
    SrsVideoCodecIdDisabled = 8

    SrsVideoCodecIdSorensonH263 = 2
    SrsVideoCodecIdScreenVideo = 3
    SrsVideoCodecIdOn2VP6 = 4
    SrsVideoCodecIdOn2VP6WithAlphaChannel = 5
    SrsVideoCodecIdScreenVideoVersion2 = 6
    SrsVideoCodecIdAVC = 7
)

/**
 * The audio codec id.
 * @doc video_file_format_spec_v10_1.pdf, page 76, E.4.2 Audio Tags
 * SoundFormat UB [4]
 * Format of SoundData. The following values are defined:
 *     0 = Linear PCM, platform endian
 *     1 = ADPCM
 *     2 = MP3
 *     3 = Linear PCM, little endian
 *     4 = Nellymoser 16 kHz mono
 *     5 = Nellymoser 8 kHz mono
 *     6 = Nellymoser
 *     7 = G.711 A-law logarithmic PCM
 *     8 = G.711 mu-law logarithmic PCM
 *     9 = reserved
 *     10 = AAC
 *     11 = Speex
 *     14 = MP3 8 kHz
 *     15 = Device-specific sound
 * Formats 7, 8, 14, and 15 are reserved.
 * AAC is supported in Flash Player 9,0,115,0 and higher.
 * Speex is supported in Flash Player 10 and higher.
 */
const (
    // set to the max value to reserved, for array map.
    SrsAudioCodecIdReserved1 = 16
    SrsAudioCodecIdForbidden = 16

    // for user to disable audio, for example, use pure video hls.
    SrsAudioCodecIdDisabled = 17

    SrsAudioCodecIdLinearPCMPlatformEndian = 0
    SrsAudioCodecIdADPCM = 1
    SrsAudioCodecIdMP3 = 2
    SrsAudioCodecIdLinearPCMLittleEndian = 3
    SrsAudioCodecIdNellymoser16kHzMono = 4
    SrsAudioCodecIdNellymoser8kHzMono = 5
    SrsAudioCodecIdNellymoser = 6
    SrsAudioCodecIdReservedG711AlawLogarithmicPCM = 7
    SrsAudioCodecIdReservedG711MuLawLogarithmicPCM = 8
    SrsAudioCodecIdReserved = 9
    SrsAudioCodecIdAAC = 10
    SrsAudioCodecIdSpeex = 11
    SrsAudioCodecIdReservedMP3_8kHz = 14
    SrsAudioCodecIdReservedDeviceSpecificSound = 15
)

/**
 * The audio sample rate.
 * @see srs_flv_srates and srs_aac_srates.
 * @doc video_file_format_spec_v10_1.pdf, page 76, E.4.2 Audio Tags
 *      0 = 5.5 kHz = 5512 Hz
 *      1 = 11 kHz = 11025 Hz
 *      2 = 22 kHz = 22050 Hz
 *      3 = 44 kHz = 44100 Hz
 * However, we can extends this table.
 */
const (
    // set to the max value to reserved, for array map.
    SrsAudioSampleRateReserved = 4
    SrsAudioSampleRateForbidden = 4

    SrsAudioSampleRate5512 = 0
    SrsAudioSampleRate11025 = 1
    SrsAudioSampleRate22050 = 2
    SrsAudioSampleRate44100 = 3
)

/**
 * The audio sample size in bits.
 * @doc video_file_format_spec_v10_1.pdf, page 76, E.4.2 Audio Tags
 * Size of each audio sample. This parameter only pertains to
 * uncompressed formats. Compressed formats always decode
 * to 16 bits internally.
 *      0 = 8-bit samples
 *      1 = 16-bit samples
 */
const (
// set to the max value to reserved, for array map.
SrsAudioSampleBitsReserved = 2
SrsAudioSampleBitsForbidden = 2

SrsAudioSampleBits8bit = 0
SrsAudioSampleBits16bit = 1
)

/**
 * The audio channels.
 * @doc video_file_format_spec_v10_1.pdf, page 77, E.4.2 Audio Tags
 * Mono or stereo sound
 *      0 = Mono sound
 *      1 = Stereo sound
 */
const (
// set to the max value to reserved, for array map.
SrsAudioChannelsReserved = 2
SrsAudioChannelsForbidden = 2

SrsAudioChannelsMono = 0
SrsAudioChannelsStereo = 1
)

/**
 * Table 7-1 - NAL unit type codes, syntax element categories, and NAL unit type classes
 * ISO_IEC_14496-10-AVC-2012.pdf, page 83.
 */
const (
    // Unspecified
    SrsAvcNaluTypeReserved = 0
    SrsAvcNaluTypeForbidden = 0

    // Coded slice of a non-IDR picture slice_layer_without_partitioning_rbsp( )
    SrsAvcNaluTypeNonIDR = 1
    // Coded slice data partition A slice_data_partition_a_layer_rbsp( )
    SrsAvcNaluTypeDataPartitionA = 2
    // Coded slice data partition B slice_data_partition_b_layer_rbsp( )
    SrsAvcNaluTypeDataPartitionB = 3
    // Coded slice data partition C slice_data_partition_c_layer_rbsp( )
    SrsAvcNaluTypeDataPartitionC = 4
    // Coded slice of an IDR picture slice_layer_without_partitioning_rbsp( )
    SrsAvcNaluTypeIDR = 5
    // Supplemental enhancement information (SEI) sei_rbsp( )
    SrsAvcNaluTypeSEI = 6
    // Sequence parameter set seq_parameter_set_rbsp( )
    SrsAvcNaluTypeSPS = 7
    // Picture parameter set pic_parameter_set_rbsp( )
    SrsAvcNaluTypePPS = 8
    // Access unit delimiter access_unit_delimiter_rbsp( )
    SrsAvcNaluTypeAccessUnitDelimiter = 9
    // End of sequence end_of_seq_rbsp( )
    SrsAvcNaluTypeEOSequence = 10
    // End of stream end_of_stream_rbsp( )
    SrsAvcNaluTypeEOStream = 11
    // Filler data filler_data_rbsp( )
    SrsAvcNaluTypeFilterData = 12
    // Sequence parameter set extension seq_parameter_set_extension_rbsp( )
    SrsAvcNaluTypeSPSExt = 13
    // Prefix NAL unit prefix_nal_unit_rbsp( )
    SrsAvcNaluTypePrefixNALU = 14
    // Subset sequence parameter set subset_seq_parameter_set_rbsp( )
    SrsAvcNaluTypeSubsetSPS = 15
    // Coded slice of an auxiliary coded picture without partitioning slice_layer_without_partitioning_rbsp( )
    SrsAvcNaluTypeLayerWithoutPartition = 19
    // Coded slice extension slice_layer_extension_rbsp( )
    SrsAvcNaluTypeCodedSliceExt = 20
)

/**
 * the aac profile, for ADTS(HLS/TS)
 * @see https://github.com/ossrs/srs/issues/310
 */
const (
    SrsAacProfileReserved = 3

    // @see 7.1 Profiles, aac-iso-13818-7.pdf, page 40
    SrsAacProfileMain = 0
    SrsAacProfileLC = 1
    SrsAacProfileSSR = 2
)

/**
 * the level for avc/h.264.
 * @see Annex A Profiles and levels, ISO_IEC_14496-10-AVC-2003.pdf, page 207.
 */
const (
    SrsAvcLevelReserved = 0

    SrsAvcLevel_1 = 10
    SrsAvcLevel_11 = 11
    SrsAvcLevel_12 = 12
    SrsAvcLevel_13 = 13
    SrsAvcLevel_2 = 20
    SrsAvcLevel_21 = 21
    SrsAvcLevel_22 = 22
    SrsAvcLevel_3 = 30
    SrsAvcLevel_31 = 31
    SrsAvcLevel_32 = 32
    SrsAvcLevel_4 = 40
    SrsAvcLevel_41 = 41
    SrsAvcLevel_5 = 50
    SrsAvcLevel_51 = 51
)

/**
 * 8.4.3.3 Semantics
 * ISO_IEC_14496-12-base-format-2012.pdf, page 37
 */
const (
    SrsMp4HandlerTypeForbidden = 0x00

    SrsMp4HandlerTypeVIDE = 0x76696465 // 'vide'
    SrsMp4HandlerTypeSOUN = 0x736f756e // 'soun'
)

// Table 1 — List of Class Tags for Descriptors
// ISO_IEC_14496-1-System-2010.pdf, page 31
const (
    SrsMp4ESTagESforbidden = 0x00
    SrsMp4ESTagESObjectDescrTag = 0x01
    SrsMp4ESTagESInitialObjectDescrTag = 0x02
    SrsMp4ESTagESDescrTag = 0x03
    SrsMp4ESTagESDecoderConfigDescrTag = 0x04
    SrsMp4ESTagESDecSpecificInfoTag = 0x05
    SrsMp4ESTagESSLConfigDescrTag = 0x06
    SrsMp4ESTagESExtSLConfigDescrTag = 0x064
)

// Table 5 — objectTypeIndication Values
// ISO_IEC_14496-1-System-2010.pdf, page 49
const (
    SrsMp4ObjectTypeForbidden = 0x00
    // Audio ISO/IEC 14496-3
    SrsMp4ObjectTypeAac = 0x40
)

// Table 6 — streamType Values
// ISO_IEC_14496-1-System-2010.pdf, page 51
const (
    SrsMp4StreamTypeForbidden = 0x00
    SrsMp4StreamTypeAudioStream = 0x05
)

/*
*********************************************************************
                                FLV enums
*********************************************************************
*/
/**
 * The frame type, for example, audio, video or data.
 * @doc video_file_format_spec_v10_1.pdf, page 75, E.4.1 FLV Tag
 */
const (
    // set to the zero to reserved, for array map.
    SrsFrameTypeReserved = 0
    SrsFrameTypeForbidden = 0

    // 8 = audio
    SrsFrameTypeAudio = 8
    // 9 = video
    SrsFrameTypeVideo = 9
    // 18 = script data
    SrsFrameTypeScript = 18
)

/**
 * The video AVC frame type, such as I/P/B.
 * @doc video_file_format_spec_v10_1.pdf, page78, E.4.3.1 VIDEODATA
 * Frame Type UB [4]
 * Type of video frame. The following values are defined:
 *      1 = key frame (for AVC, a seekable frame)
 *      2 = inter frame (for AVC, a non-seekable frame)
 *      3 = disposable inter frame (H.263 only)
 *      4 = generated key frame (reserved for server use only)
 *      5 = video info/command frame
 */
const (
    // set to the zero to reserved, for array map.
    SrsVideoAvcFrameTypeReserved = 0
    SrsVideoAvcFrameTypeForbidden = 0
    SrsVideoAvcFrameTypeReserved1 = 6

    SrsVideoAvcFrameTypeKeyFrame = 1
    SrsVideoAvcFrameTypeInterFrame = 2
    SrsVideoAvcFrameTypeDisposableInterFrame = 3
    SrsVideoAvcFrameTypeGeneratedKeyFrame = 4
    SrsVideoAvcFrameTypeVideoInfoFrame = 5
)

/**
 * The video AVC frame trait(characteristic).
 * @doc video_file_format_spec_v10_1.pdf, page79, E.4.3.2 AVCVIDEOPACKET
 * AVCPacketType IF CodecID == 7 UI8
 * The following values are defined:
 *      0 = AVC sequence header
 *      1 = AVC NALU
 *      2 = AVC end of sequence (lower level NALU sequence ender is not required or supported)
 */
const (
    // set to the max value to reserved, for array map.
    SrsVideoAvcFrameTraitReserved = 3
    SrsVideoAvcFrameTraitForbidden = 3

    SrsVideoAvcFrameTraitSequenceHeader = 0
    SrsVideoAvcFrameTraitNALU = 1
    SrsVideoAvcFrameTraitSequenceHeaderEOF = 2
)

/**
 * The audio AAC frame trait(characteristic).
 * @doc video_file_format_spec_v10_1.pdf, page 77, E.4.2 Audio Tags
 * AACPacketType IF SoundFormat == 10 UI8
 * The following values are defined:
 *      0 = AAC sequence header
 *      1 = AAC raw
 */
const (
    // set to the max value to reserved, for array map.
    SrsAudioAacFrameTraitReserved = 2
    SrsAudioAacFrameTraitForbidden = 2

    SrsAudioAacFrameTraitSequenceHeader = 0
    SrsAudioAacFrameTraitRawData = 1
)

/**
* E.4.1 FLV Tag, page 75
*/
const (
    SRS_RTMP_TYPE_AUDIO = 8
    SRS_RTMP_TYPE_VIDEO = 9
    SRS_RTMP_TYPE_SCRIPT = 18
)

