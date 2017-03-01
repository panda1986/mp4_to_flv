package main

import "encoding/binary"

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
