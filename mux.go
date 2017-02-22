package main

import (
    "os"
    ol "github.com/ossrs/go-oryx-lib/logger"
    "fmt"
)

func Mux(mp4, flv *os.File) (err error)  {
    ol.T(nil, fmt.Sprint("start ingest mp4 to flv."))

    dec := NewMp4Decoder()
    if err = dec.Init(mp4); err != nil {
        ol.E(nil, fmt.Sprintf("init mp4 decoder failed, err is %v", err))
        return
    }
    ol.T(nil, fmt.Sprintf("dec:%+v", dec))

    /*for {

        // Read a mp4 sample and convert to flv tag

    }*/
    return
}
