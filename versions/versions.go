package main

import (
	"log"

	"github.com/skemtoputaete/goav/avcodec"
	"github.com/skemtoputaete/goav/avdevice"
	"github.com/skemtoputaete/goav/avfilter"
	"github.com/skemtoputaete/goav/avutil"
	"github.com/skemtoputaete/goav/swresample"
	"github.com/skemtoputaete/goav/swscale"
)

func main() {
	log.Printf("AvFilter Version:\t%v", avfilter.AvfilterVersion())
	log.Printf("AvDevice Version:\t%v", avdevice.AvdeviceVersion())
	log.Printf("SWScale Version:\t%v", swscale.SwscaleVersion())
	log.Printf("AvUtil Version:\t%v", avutil.AvutilVersion())
	log.Printf("AvCodec Version:\t%v", avcodec.AvcodecVersion())
	log.Printf("Resample Version:\t%v", swresample.SwresampleLicense())
}
