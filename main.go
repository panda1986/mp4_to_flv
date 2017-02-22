package main

import (
    ol "github.com/ossrs/go-oryx-lib/logger"
    "fmt"
    "flag"
    "os"
)

const (
    version string = "0.0.1"
)

func main()  {
    ol.T(nil, fmt.Sprintf("mp4 to flv parser:%v, by panda of bravovcloud.com", version))

    var mp4Url, flvUrl string
    flag.StringVar(&mp4Url, "i", "./test.mp4", "input mp4 file to be parsed")
    flag.StringVar(&flvUrl, "y", "./test.flv", "output flv file")

    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
        flag.PrintDefaults()
    }

    flag.Parse()

    ol.T(nil, fmt.Sprintf("the input mp4 url is: %v, output flv is:%v", mp4Url, flvUrl))

    var f *os.File
    var err error
    if f, err = os.Open(mp4Url); err != nil {
        ol.E(nil, fmt.Sprintf("open mp4 file failed, err is %v", err))
        return
    }
    defer f.Close()

    var flv *os.File
    if flv, err = os.Create(flvUrl); err != nil {
        ol.E(nil, fmt.Sprintf("create flv file failed, err is %v", err))
        return
    }
    defer flv.Close()

    if err = Mux(f, flv); err != nil {
        ol.E(nil, fmt.Sprintf("mux mp4:%v to flv:%v failed, err is %v", mp4Url, flvUrl, err))
        return
    }

    ol.T(nil, fmt.Sprintf("ingest mp4 to flv ok."))
    return
}
