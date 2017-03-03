package main

import (
    "encoding/binary"
    "os"
    "fmt"
    ol "github.com/ossrs/go-oryx-lib/logger"
)

// intDataSize returns the size of the data required to represent the data when encoded.
// It returns zero if the type cannot be implemented by the fast path in Read or Write.
func uint64DataSize(data interface{}) uint64 {
    switch data.(type) {
    case int8, uint8, *int8, *uint8:
        return uint64(1)
    case int16, uint16, *int16, *uint16:
        return uint64(2)
    case int32, uint32, *int32, *uint32:
        return uint64(4)
    case int64, uint64, *int64, *uint64:
        return uint64(8)
    case []uint8:
        arru8 := data.([]uint8)
        return uint64(len(arru8))
    }
    return 0
}

func Bytes3ToUint32(b []byte) uint32 {
    nb := []byte{}
    nb = append(nb, 0)
    nb = append(nb, b...)
    return binary.BigEndian.Uint32(nb)
}

func max(x, y int32) int32 {
    if x > y {
        return x
    }
    return y
}

func min(x, y int32) int32 {
    if x > y {
        return y
    }
    return x
}

func readAt(mp4 string, offset int64, length int) (data []byte, err error) {
    var f *os.File
    if f, err = os.Open(mp4); err != nil {
        ol.E(nil, fmt.Sprintf("open mp4 file failed, err is %v", err))
        return
    }
    defer f.Close()

    data = make([]byte, length)
    var n int
    if n, err = f.ReadAt(data, offset); err != nil {
        return
    }
    if n != length {
        err = fmt.Errorf("read at offset:%x, exp len=%v, actual len=%v", offset, length, n)
        ol.E(nil, err.Error())
        return
    }

    return
}

//TODO: need complete
func to3Bytes(from uint32) (to []byte) {
    to = make([]byte, 3)
    return
}