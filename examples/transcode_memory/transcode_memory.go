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
	"io"
	"log"
	"os"
	"unsafe"

	"github.com/skemtoputaete/goav/avcodec"
	"github.com/skemtoputaete/goav/avformat"
	"github.com/skemtoputaete/goav/avutil"
	"github.com/skemtoputaete/goav/swresample"
)

const (
	/* The output bit rate in bit/s */
	OUTPUT_BIT_RATE = 96000
	/* The number of output channels */
	OUTPUT_CHANNELS = 2
	/* Buffer size */
	BUF_SIZE = 32768
)

var pts int64

func initPacketBuf(buf_size int) *avformat.AvioCustomBuffer {
	// Buffer for read-callback
	avio_ctx_buffer := bytes.NewBuffer(make([]byte, buf_size))
	// Custom buffer struct for read-callback
	return &avformat.AvioCustomBuffer{Buffer: avio_ctx_buffer}
}

// Initialize AVProbeData for AVFormatContext
func initAvProbeData(buf *uint8, buf_size int, pd_filename *C.char) *avformat.AvProbeData {
	var av_probe_data avformat.AvProbeData
	av_probe_data.SetBuf(buf)
	av_probe_data.SetBufSize(buf_size)
	av_probe_data.SetFilename((*avformat.CString)(pd_filename))
	return &av_probe_data
}

func openInput(packet_buf *avformat.AvioCustomBuffer, avio_buf *uint8, buf_size int, pd_filename *C.char) (int, *avformat.Context, *avcodec.Context) {
	ret := 0

	ifmt_ctx := avformat.AvformatAllocContext()
	// Create AVIOContext
	avio_ctx := avformat.AvioAllocContext(ifmt_ctx, packet_buf, avio_buf, buf_size, 0)
	// Set AvIOContext to AVFormatContext
	ifmt_ctx.SetPb(avio_ctx)
	av_probe_data := initAvProbeData(avio_buf, buf_size, pd_filename)
	// Set AVProbeData to AVFormatContext
	ifmt_ctx.SetIformat(avformat.AvProbeInputFormat(av_probe_data, 1))
	ifmt_ctx.AddFlag(avformat.AVFMT_FLAG_CUSTOM_IO)

	if avformat.AvformatOpenInput(&ifmt_ctx, "", nil, nil) != 0 {
		fmt.Fprintf(os.Stderr, "Unable to open input\n")
		ret = avutil.AVERROR_EXIT
		return ret, nil, nil
	}

	if ifmt_ctx.AvformatFindStreamInfo(nil) < 0 {
		fmt.Fprintf(os.Stderr, "Couldn't find stream information")
		ret = avutil.AVERROR_EXIT
		return ret, nil, nil
	}
	ifmt_ctx.AvDumpFormat(0, "", 0)

	if ifmt_ctx.NbStreams() != 1 {
		fmt.Fprintf(os.Stderr, "Expected one audio input stream, but found %d\n", ifmt_ctx.NbStreams())
		ret = avutil.AVERROR_EXIT
		return ret, nil, nil
	}

	codec_id := ifmt_ctx.Streams()[0].CodecParameters().CodecId()
	input_codec := avcodec.AvcodecFindDecoder(codec_id)

	if input_codec == nil {
		fmt.Fprintf(os.Stderr, "Could not find input codec\n")
		ret = avutil.AVERROR_EXIT
		return ret, nil, nil
	}
	av_ctx := input_codec.AvcodecAllocContext3()
	if av_ctx == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate a decoding context\n")
		ret = avutil.AVERROR_ENOMEM
		return ret, nil, nil
	}
	defer func() {
		if ret < 0 {
			avcodec.AvcodecFreeContext(av_ctx)
		}
	}()
	ret = avcodec.AvcodecParametersToContext(av_ctx, ifmt_ctx.Streams()[0].CodecParameters())
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not get codec parameters\n")
		return ret, nil, nil
	}
	if ret = av_ctx.AvcodecOpen2(input_codec, nil); ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not open input codec (error '%s')\n", avutil.AvStrerr(ret))
		return ret, nil, nil
	}

	return 0, ifmt_ctx, av_ctx
}

func openOutputFile(filename string, icodec_ctx *avcodec.Context) (int, *avformat.Context, *avcodec.Context) {
	ret := 0

	ofmtCtx := avformat.AvformatAllocContext()
	if avformat.AvformatAllocOutputContext2(&ofmtCtx, nil, "", filename) != 0 {
		fmt.Fprintf(os.Stderr, "Unable to alloc output context for %s\n", filename)
		ret = avutil.AVERROR_ENOMEM
		return ret, nil, nil
	}
	defer func() {
		if ret < 0 {
			ofmtCtx.AvformatFreeContext()
		}
	}()

	pb := (*avformat.AvIOContext)(nil)
	ret = avformat.AvIOOpen(&pb, filename, avformat.AVIO_FLAG_WRITE)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not open output file '%s'\n", filename)
		ret = avutil.AVERROR_EXIT
		return ret, nil, nil
	}
	ofmtCtx.SetPb(pb)
	defer func() {
		if ret < 0 {
			pb := ofmtCtx.Pb()
			avformat.AvIOClosep(&pb)
		}
	}()

	outputCodec := avcodec.AvcodecFindEncoder(avcodec.CodecId(avcodec.AV_CODEC_ID_AAC))
	if outputCodec == nil {
		fmt.Fprintf(os.Stderr, "Could not find an AAC encoder\n")
		ret = avutil.AVERROR_EXIT
		return ret, nil, nil
	}
	outStream := ofmtCtx.AvformatNewStream(nil)
	if outStream == nil {
		fmt.Fprintf(os.Stderr, "Could not reate new stream\n")
		ret = avutil.AVERROR_ENOMEM
		return ret, nil, nil
	}
	avCtx := outputCodec.AvcodecAllocContext3()
	if avCtx == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate an encoding context\n")
		ret = avutil.AVERROR_ENOMEM
		return ret, nil, nil
	}
	defer func() {
		if ret < 0 {
			avcodec.AvcodecFreeContext(avCtx)
		}
	}()

	/* Set the basic encoder parameters.
	 * The input file's sample rate is used to avoid a sample rate conversion. */
	avCtx.SetChannels(OUTPUT_CHANNELS)
	avCtx.SetChannelLayout(avutil.AvGetDefaultChannelLayout(OUTPUT_CHANNELS))
	avCtx.SetSampleRate(icodec_ctx.SampleRate())
	avCtx.SetSampleFmt(outputCodec.SampleFmts()[0])
	avCtx.SetBitRate(OUTPUT_BIT_RATE)

	/* Allow the use of the experimental AAC encoder. */
	avCtx.SetStrictStdCompliance(avcodec.FF_COMPLIANCE_EXPERIMENTAL)

	/* Set the sample rate for the container. */
	outStream.SetTimeBase(avutil.NewRational(1, icodec_ctx.SampleRate()))

	/* Some container formats (like MP4) require global headers to be present.
	 * Mark the encoder so that it behaves accordingly. */
	if (ofmtCtx.Oformat().Flags() & avformat.AVFMT_GLOBALHEADER) != 0 {
		avCtx.SetFlags(avCtx.Flags() | avcodec.AV_CODEC_FLAG_GLOBAL_HEADER)
	}

	if ret = avCtx.AvcodecOpen2(outputCodec, nil); ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not open output codec (error '%s')\n", avutil.AvStrerr(ret))
		return ret, nil, nil
	}
	ret = avcodec.AvcodecParametersFromContext(outStream.CodecParameters(), avCtx)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not initialize stream parameters\n")
		return ret, nil, nil
	}

	return 0, ofmtCtx, avCtx
}

func initResampler(icodec_ctx, ocodec_ctx *avcodec.Context) (int, *swresample.Context) {
	resampleCtx := swresample.SwrAllocSetOpts(
		int64(ocodec_ctx.ChannelLayout()),
		swresample.AvSampleFormat(ocodec_ctx.SampleFmt()),
		ocodec_ctx.SampleRate(),
		int64(icodec_ctx.ChannelLayout()),
		swresample.AvSampleFormat(icodec_ctx.SampleFmt()),
		icodec_ctx.SampleRate())
	if resampleCtx == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate resample context\n")
		return avutil.AVERROR_ENOMEM, nil
	}
	if ocodec_ctx.SampleRate() != icodec_ctx.SampleRate() {
		fmt.Fprintf(os.Stderr, "sample rate is not the same\n")
		return avutil.AVERROR_EXIT, nil
	}
	ret := resampleCtx.SwrInit()
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not open resample context\n")
		resampleCtx.SwrFree()
		return ret, nil
	}
	return 0, resampleCtx
}

func initFifo(ocodec_ctx *avcodec.Context) (int, *avutil.AvAudioFifo) {
	fifo := avutil.AvAudioFifoAlloc(
		avutil.AvSampleFormat(ocodec_ctx.SampleFmt()),
		ocodec_ctx.Channels(), 1)
	if fifo == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate FIFO\n")
		return avutil.AVERROR_ENOMEM, nil
	}
	return 0, fifo
}

func writeOutputFileHeader(ofmt_ctx *avformat.Context) int {
	ret := ofmt_ctx.AvformatWriteHeader(nil)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not write output file header (error '%s')\n", avutil.AvStrerr(ret))
		return ret
	}
	return 0
}

func writeOutputFileTrailer(ofmt_ctx *avformat.Context) int {
	ret := ofmt_ctx.AvWriteTrailer()
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not write output file trailer (error '%s')\n", avutil.AvStrerr(ret))
		return ret
	}
	return 0
}

func initInputFrame() (int, *avutil.Frame) {
	frame := avutil.AvFrameAlloc()
	if frame == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate input frame\n")
		return avutil.AVERROR_ENOMEM, nil
	}
	return 0, frame
}

func decodeAudioFrame(frame *avutil.Frame, inputFormatCtx *avformat.Context, inputCodecCtx *avcodec.Context) (ret int, dataPresent, finished bool) {
	packet := avcodec.AvPacketAlloc()
	defer avcodec.AvPacketFree(packet)

	ret = inputFormatCtx.AvReadFrame(packet)
	if ret < 0 {
		if ret == avutil.AVERROR_EOF {
			finished = true
		} else {
			fmt.Fprintf(os.Stderr, "Could not read frame (error '%s')\n", avutil.AvStrerr(ret))
			return ret, false, false
		}
	}
	ret = avcodec.AvcodecSendPacket(inputCodecCtx, packet)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not send packet for decoding (error '%s')\n", avutil.AvStrerr(ret))
		return ret, false, false
	}
	ret = avcodec.AvcodecReceiveFrame(inputCodecCtx, frame)
	if ret == avutil.AVERROR_EAGAIN {
		return 0, false, false
	} else if ret == avutil.AVERROR_EOF {
		return 0, false, true
	} else if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not decode frame (error '%s')\n", avutil.AvStrerr(ret))
		return ret, false, false
	} else {
		return 0, true, false
	}
}

func initConvertedSamples(converted_input_samples ***uint8, outputCodecCtx *avcodec.Context, frameSize int) int {
	ret := avutil.AvSamplesAllocArrayAndSamples(converted_input_samples, nil, avutil.AvGetChannelLayoutNbChannels(outputCodecCtx.ChannelLayout()), frameSize, int(outputCodecCtx.SampleFmt()), 0)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not allocate converted input samples (error '%s')\n", avutil.AvStrerr(ret))
		return ret
	}
	return 0
}

func readDecodeConvertAndStore(fifo *avutil.AvAudioFifo, inputFormatCtx *avformat.Context,
	inputCodecCtx *avcodec.Context, outputCodecCtx *avcodec.Context, resampleCtx *swresample.Context) (int, bool) {
	ret, inputFrame := initInputFrame()
	if ret < 0 {
		return ret, false
	}
	defer avutil.AvFrameFree(inputFrame)

	ret, dataPresent, finished := decodeAudioFrame(inputFrame, inputFormatCtx, inputCodecCtx)
	if ret < 0 {
		return ret, false
	}
	if finished {
		return 0, true
	}
	if !dataPresent {
		return 0, false
	}
	frameSize := inputFrame.NbSamples()
	// fmt.Fprintf(os.Stdout, "Input frame size = %d\n", frameSize)
	var convertedInputSamples **uint8 = nil
	ret = initConvertedSamples(&convertedInputSamples, outputCodecCtx, frameSize)
	if ret < 0 {
		return ret, false
	}
	defer avutil.AvSamplesFreeArrayAndSamples(convertedInputSamples)

	ret = convertSamples((**uint8)(inputFrame.ExtendedData()), convertedInputSamples, frameSize, resampleCtx)
	if ret < 0 {
		return ret, false
	}
	ret = addSamplesToFifo(fifo, convertedInputSamples, frameSize)
	if ret < 0 {
		return ret, false
	}
	return 0, false
}

func convertSamples(inputData, convertedData **uint8, frameSize int, resampleCtx *swresample.Context) int {
	ret := resampleCtx.SwrConvert(convertedData, frameSize, inputData, frameSize)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not convert input samples (error '%s')\n", avutil.AvStrerr(ret))
		return ret
	}
	return 0
}

func addSamplesToFifo(fifo *avutil.AvAudioFifo, convertedInputSamples **uint8, frameSize int) int {
	ret := fifo.AvAudioFifoRealloc(fifo.AvAudioFifoSize() + frameSize)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not reallocate FIFO\n")
		return ret
	}
	if fifo.AvAudioFifoWrite(convertedInputSamples, frameSize) < frameSize {
		fmt.Fprintf(os.Stderr, "Could not write data to FIFO\n")
		return avutil.AVERROR_EXIT
	}
	return 0
}

func loadEncodeAndWrite(fifo *avutil.AvAudioFifo, outputFormatCtx *avformat.Context, outputCodecCtx *avcodec.Context) int {
	frameSize := fifo.AvAudioFifoSize()
	if frameSize > outputCodecCtx.FrameSize() {
		frameSize = outputCodecCtx.FrameSize()
	}
	ret, outputFrame := initOutputFrame(outputCodecCtx, frameSize)
	if ret < 0 {
		return ret
	}
	defer avutil.AvFrameFree(outputFrame)
	if fifo.AvAudioFifoRead(outputFrame.GetDataP(), frameSize) < frameSize {
		fmt.Fprintf(os.Stderr, "Could not read data from FIFO\n")
		return avutil.AVERROR_EXIT
	}
	ret, _ = encodeAudioFrame(outputFrame, outputFormatCtx, outputCodecCtx)
	log.Println("Audio frame encoded")
	if ret < 0 {
		return ret
	}
	return 0
}

func encodeAudioFrame(frame *avutil.Frame, outputFormatCtx *avformat.Context, outputCodecCtx *avcodec.Context) (int, bool) {
	outputPacket := avcodec.AvPacketAlloc()
	defer avcodec.AvPacketFree(outputPacket)

	if frame != nil {
		frame.SetPts(pts)
		pts += int64(frame.NbSamples())
	}
	ret := avcodec.AvcodecSendFrame(outputCodecCtx, frame)
	if ret == avutil.AVERROR_EOF {
		return 0, false
	} else if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not send packet for encoding (error '%s')\n", avutil.AvStrerr(ret))
		return ret, false
	}
	ret = avcodec.AvcodecReceivePacket(outputCodecCtx, outputPacket)
	if ret == avutil.AVERROR_EAGAIN {
		return 0, false
	} else if ret == avutil.AVERROR_EOF {
		return 0, false
	} else if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not encode frame (error '%s')\n", avutil.AvStrerr(ret))
		return ret, false
	}
	ret = outputFormatCtx.AvWriteFrame(outputPacket)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not write frame (error '%s')\n", avutil.AvStrerr(ret))
		return ret, false
	}
	return 0, true
}

func initOutputFrame(outputCodecCtx *avcodec.Context, frameSize int) (int, *avutil.Frame) {
	frame := avutil.AvFrameAlloc()
	if frame == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate output frame\n")
		return avutil.AVERROR_EXIT, nil
	}
	frame.SetNbSamples(frameSize)
	frame.SetChannelLayout(outputCodecCtx.ChannelLayout())
	frame.SetFormat(int(outputCodecCtx.SampleFmt()))
	frame.SetSampleRate(outputCodecCtx.SampleRate())

	ret := avutil.AvFrameGetBuffer(frame, 0)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not allocate output frame samples (error '%s')\n", avutil.AvStrerr(ret))
		avutil.AvFrameFree(frame)
		return ret, nil
	}
	return 0, frame
}

func TranscodeAudio(filename string) int {
	file_data, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	file_buffer := bytes.NewBuffer(file_data)
	temp_buffer := make([]byte, BUF_SIZE)

	// Buffer for AVIOContext
	avio_read_buffer := (*uint8)(avutil.AvMalloc(BUF_SIZE))
	packet_buf := initPacketBuf(0)

	// Fill buffers
	file_buffer.Read(temp_buffer)
	packet_buf.WriteBuffer(temp_buffer)

	pd_filename := C.CString("")
	defer C.free(unsafe.Pointer(pd_filename))

	ret, ifmt_ctx, icodec_ctx := openInput(packet_buf, avio_read_buffer, BUF_SIZE, pd_filename)
	if ret < 0 {
		return ret
	}
	defer avformat.AvformatCloseInput(ifmt_ctx)
	// bytes.Buffer does not allow to seek. Fill buffer with initial data
	// packet_buf.WriteBuffer(temp_buffer)

	log.Println("Start transcoding")

	iteration := 0

	for {

		log.Printf("Transcoding iteration: %d. Buffer length: %d \n", iteration, packet_buf.Buffer.Len())

		if iteration != 0 {
			read_bytes, err := file_buffer.Read(temp_buffer)
			if err == io.EOF {
				break
			}

			packet_buf.WriteBuffer(temp_buffer)
			log.Printf("Read %d bytes from file. Buffer length %d \n", read_bytes, packet_buf.Buffer.Len())
		}

		// C.memcpy(unsafe.Pointer(avio_read_buffer), unsafe.Pointer(&temp_buffer[0]), C.size_t(BUF_SIZE))

		ret, ofmt_ctx, ocodec_ctx := openOutputFile(fmt.Sprintf("chunk-%d.aac", iteration), icodec_ctx)
		if ret < 0 {
			return ret
		}
		ret, resampleCtx := initResampler(icodec_ctx, ocodec_ctx)
		if ret < 0 {
			return ret
		}
		ret, fifo := initFifo(ocodec_ctx)
		if ret < 0 {
			return ret
		}

		ret = writeOutputFileHeader(ofmt_ctx)
		if ret < 0 {
			return ret
		}
		log.Println("All variables initialized")

		for {
			finished := false
			outputFrameSize := ocodec_ctx.FrameSize()
			// fmt.Fprintf(os.Stdout, "Output frame size = %d \n", outputFrameSize)
			// fmt.Fprintf(os.Stdout, "FIFO size = %d \n", fifo.AvAudioFifoSize())
			for fifo.AvAudioFifoSize() < outputFrameSize {
				ret, finished = readDecodeConvertAndStore(fifo, ifmt_ctx, icodec_ctx, ocodec_ctx, resampleCtx)
				if ret < 0 {
					os.Exit(-ret)
				}
				if finished {
					fmt.Fprintf(os.Stdout, "Finish decoding \n")
					break
				}
			}
			for fifo.AvAudioFifoSize() >= outputFrameSize ||
				(finished && fifo.AvAudioFifoSize() > 0) {
				log.Printf("Encode frames. Finished - %t", finished)
				ret = loadEncodeAndWrite(fifo, ofmt_ctx, ocodec_ctx)
				if ret < 0 {
					return ret
				}
			}
			if finished {
				dataWritten := true
				for dataWritten {
					log.Println("Write encoded data")
					ret, dataWritten = encodeAudioFrame(nil, ofmt_ctx, ocodec_ctx)
					if ret < 0 {
						return ret
					}
				}
				break
			}
		}

		writeOutputFileTrailer(ofmt_ctx)
		fifo.AvAudioFifoFree()
		resampleCtx.SwrFree()
		avcodec.AvcodecFreeContext(ocodec_ctx)
		pb := ofmt_ctx.Pb()
		avformat.AvIOClosep(&pb)
		ofmt_ctx.AvformatFreeContext()

		iteration += 1
	}

	return 0
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input file>\n", os.Args[0])
		os.Exit(1)
	}
	TranscodeAudio(os.Args[1])
}
