package main

//#cgo pkg-config: libavformat libavcodec libavutil libavdevice libavfilter libswresample libswscale
//#include <stdio.h>
//#include <stdlib.h>
//#include <inttypes.h>
//#include <stdint.h>
//#include <string.h>
import "C"

import (
	"bytes"
	"fmt"
	"os"
	"unsafe"

	"github.com/skemtoputaete/goav/avformat"
	"github.com/skemtoputaete/goav/avutil"
)

const (
	/* The output bit rate in bit/s */
	OUTPUT_BIT_RATE = 96000
	/* The number of output channels */
	OUTPUT_CHANNELS = 2
	/* Buffer size */
	BUF_SIZE = 32768
)

var avio_read_buffer *uint8 = nil

func ReadInfo(filename string) {
	file_data, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	ifmt_ctx := avformat.AvformatAllocContext()

	file_buffer := bytes.NewBuffer(file_data)
	temp_buffer := make([]byte, BUF_SIZE)

	// Buffer for AVIOContext
	avio_read_buffer = (*uint8)(avutil.AvMalloc(BUF_SIZE))
	// Buffer for read-callback
	avio_ctx_buffer := bytes.NewBuffer(make([]byte, BUF_SIZE))
	// Custom buffer struct for read-callback
	packet_buf := &avformat.AvioCustomBuffer{Buffer: avio_ctx_buffer}

	// Fill buffers
	file_buffer.Read(temp_buffer)
	C.memcpy(unsafe.Pointer(avio_read_buffer), unsafe.Pointer(&temp_buffer[0]), C.size_t(BUF_SIZE))
	packet_buf.WriteBuffer(temp_buffer)

	// Create AVIOContext
	avio_ctx := avformat.AvioAllocContext(ifmt_ctx, packet_buf, avio_read_buffer, BUF_SIZE, 0)
	defer func() {
		// avutil.AvFreep(unsafe.Pointer(avio_ctx.))
		// avformat.AvioContextFree(&avio_ctx)
	}()
	// Set AvIOContext to AVFormatContext
	ifmt_ctx.SetPb(avio_ctx)

	// Initialize AVProbeData for AVFormatContext
	var av_probe_data avformat.AvProbeData
	av_probe_data.SetBuf(avio_read_buffer)
	av_probe_data.SetBufSize(BUF_SIZE)
	pd_filename := C.CString("")
	defer C.free(unsafe.Pointer(pd_filename))
	av_probe_data.SetFilename((*avformat.CString)(pd_filename))

	// Set AVProbeData to AVFormatContext
	ifmt_ctx.SetIformat(avformat.AvProbeInputFormat(&av_probe_data, 1))
	ifmt_ctx.AddFlag(avformat.AVFMT_FLAG_CUSTOM_IO)

	if ret := avformat.AvformatOpenInput(&ifmt_ctx, "", nil, nil); ret != 0 {
		fmt.Fprintf(os.Stderr, "Unable to open input. Error: %d \n", ret)
		return
	}
	defer func() {
		avformat.AvformatCloseInput(ifmt_ctx)
	}()

	if ifmt_ctx.AvformatFindStreamInfo(nil) < 0 {
		fmt.Fprintf(os.Stderr, "Couldn't find stream information")
		return
	}
	ifmt_ctx.AvDumpFormat(0, "", 0)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input file>\n", os.Args[0])
		os.Exit(1)
	}
	ReadInfo(os.Args[1])
}
