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

var pts int64 = 0

func initPacketBuf(bufSize int) *avformat.AvioCustomBuffer {
	// Buffer for read-callback
	avioCtxBuffer := bytes.NewBuffer(make([]byte, bufSize))
	// Custom buffer struct for read-callback
	return &avformat.AvioCustomBuffer{Buffer: avioCtxBuffer}
}

// Initialize AVProbeData for AVFormatContext
func initAvProbeData(buf *uint8, bufSize int, pdFilename *C.char) *avformat.AvProbeData {
	var avProbeData avformat.AvProbeData
	avProbeData.SetBuf(buf)
	avProbeData.SetBufSize(bufSize)
	avProbeData.SetFilename((*avformat.CString)(pdFilename))
	return &avProbeData
}

func openInput(packetBuffer *avformat.AvioCustomBuffer, avioBuf *uint8, bufSize int, pdFilename *C.char) (int, *avformat.Context, *avcodec.Context) {
	ret := 0

	inputFormatCtx := avformat.AvformatAllocContext()
	// Create AVIOContext
	avio_ctx := avformat.AvioAllocContext(inputFormatCtx, packetBuffer, avioBuf, bufSize, 0)
	// Set AvIOContext to AVFormatContext
	inputFormatCtx.SetPb(avio_ctx)
	inputFormatCtx.AddFlag(avformat.AVFMT_FLAG_CUSTOM_IO)
	avProbeData := initAvProbeData(avioBuf, bufSize, pdFilename)
	// Set AVProbeData to AVFormatContext
	inputFormatCtx.SetIformat(avformat.AvProbeInputFormat(avProbeData, 1))

	if avformat.AvformatOpenInput(&inputFormatCtx, "", nil, nil) != 0 {
		log.Println("Unable to open input")
		ret = avutil.AVERROR_EXIT
		return ret, nil, nil
	}

	if inputFormatCtx.AvformatFindStreamInfo(nil) < 0 {
		fmt.Fprintf(os.Stderr, "Couldn't find stream information")
		ret = avutil.AVERROR_EXIT
		return ret, nil, nil
	}
	inputFormatCtx.AvDumpFormat(0, "", 0)

	if inputFormatCtx.NbStreams() != 1 {
		fmt.Fprintf(os.Stderr, "Expected one audio input stream, but found %d\n", inputFormatCtx.NbStreams())
		ret = avutil.AVERROR_EXIT
		return ret, nil, nil
	}

	codecId := inputFormatCtx.Streams()[0].CodecParameters().CodecId()
	inputCodec := avcodec.AvcodecFindDecoder(codecId)

	if inputCodec == nil {
		fmt.Fprintf(os.Stderr, "Could not find input codec\n")
		ret = avutil.AVERROR_EXIT
		return ret, nil, nil
	}
	avCtx := inputCodec.AvcodecAllocContext3()
	if avCtx == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate a decoding context\n")
		ret = avutil.AVERROR_ENOMEM
		return ret, nil, nil
	}
	defer func() {
		if ret < 0 {
			avcodec.AvcodecFreeContext(avCtx)
		}
	}()
	ret = avcodec.AvcodecParametersToContext(avCtx, inputFormatCtx.Streams()[0].CodecParameters())
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not get codec parameters\n")
		return ret, nil, nil
	}
	if ret = avCtx.AvcodecOpen2(inputCodec, nil); ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not open input codec (error '%s')\n", avutil.AvStrerr(ret))
		return ret, nil, nil
	}

	return 0, inputFormatCtx, avCtx
}

func openOutputFile(inputCodecCtx *avcodec.Context) (int, *avformat.Context, *avcodec.Context) {
	ret := 0

	ofmtCtx := avformat.AvformatAllocContext()
	if avformat.AvformatAllocOutputContext2(&ofmtCtx, nil, "adts", "") != 0 {
		fmt.Fprintf(os.Stderr, "Unable to alloc output context\n")
		ret = avutil.AVERROR_ENOMEM
		return ret, nil, nil
	}
	defer func() {
		if ret < 0 {
			ofmtCtx.AvformatFreeContext()
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
	avCtx.SetSampleRate(inputCodecCtx.SampleRate())
	avCtx.SetSampleFmt(outputCodec.SampleFmts()[0])
	avCtx.SetBitRate(OUTPUT_BIT_RATE)

	/* Allow the use of the experimental AAC encoder. */
	avCtx.SetStrictStdCompliance(avcodec.FF_COMPLIANCE_EXPERIMENTAL)

	/* Set the sample rate for the container. */
	outStream.SetTimeBase(avutil.NewRational(1, inputCodecCtx.SampleRate()))

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

func createCodecCtx(avFmtCtx *avformat.Context, codecParameters *avcodec.CodecParameters, sampleRate int) (int, *avcodec.Context) {
	ret := 0

	outputCodec := avcodec.AvcodecFindEncoder(avcodec.CodecId(avcodec.AV_CODEC_ID_AAC))
	if outputCodec == nil {
		fmt.Fprintf(os.Stderr, "Could not find an AAC encoder\n")
		ret = avutil.AVERROR_EXIT
		return ret, nil
	}
	avCtx := outputCodec.AvcodecAllocContext3()
	if avCtx == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate an encoding context\n")
		ret = avutil.AVERROR_ENOMEM
		return ret, nil
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
	avCtx.SetSampleRate(sampleRate)
	avCtx.SetSampleFmt(outputCodec.SampleFmts()[0])
	avCtx.SetBitRate(OUTPUT_BIT_RATE)

	/* Allow the use of the experimental AAC encoder. */
	avCtx.SetStrictStdCompliance(avcodec.FF_COMPLIANCE_EXPERIMENTAL)

	/* Some container formats (like MP4) require global headers to be present.
	 * Mark the encoder so that it behaves accordingly. */
	if (avFmtCtx.Oformat().Flags() & avformat.AVFMT_GLOBALHEADER) != 0 {
		avCtx.SetFlags(avCtx.Flags() | avcodec.AV_CODEC_FLAG_GLOBAL_HEADER)
	}

	if ret = avCtx.AvcodecOpen2(outputCodec, nil); ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not open output codec (error '%s')\n", avutil.AvStrerr(ret))
		return ret, nil
	}
	ret = avcodec.AvcodecParametersFromContext(codecParameters, avCtx)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not initialize stream parameters\n")
		return ret, nil
	}

	return 0, avCtx
}

func openFile(filename string, fmtCtx *avformat.Context) (int, *avformat.AvIOContext) {
	pb := (*avformat.AvIOContext)(nil)
	ret := avformat.AvIOOpen(&pb, filename, avformat.AVIO_FLAG_WRITE)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not open output file '%s'\n", filename)
		ret = avutil.AVERROR_EXIT
		return ret, nil
	}
	return 0, pb
}

func initResampler(inputCodecCtx, outputCodecCtx *avcodec.Context) (int, *swresample.Context) {
	resampleCtx := swresample.SwrAllocSetOpts(
		int64(outputCodecCtx.ChannelLayout()),
		swresample.AvSampleFormat(outputCodecCtx.SampleFmt()),
		outputCodecCtx.SampleRate(),
		int64(inputCodecCtx.ChannelLayout()),
		swresample.AvSampleFormat(inputCodecCtx.SampleFmt()),
		inputCodecCtx.SampleRate())
	if resampleCtx == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate resample context\n")
		return avutil.AVERROR_ENOMEM, nil
	}
	if outputCodecCtx.SampleRate() != inputCodecCtx.SampleRate() {
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

func initFifo(outputCodecCtx *avcodec.Context) (int, *avutil.AvAudioFifo) {
	fifo := avutil.AvAudioFifoAlloc(
		avutil.AvSampleFormat(outputCodecCtx.SampleFmt()),
		outputCodecCtx.Channels(), 1)
	if fifo == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate FIFO\n")
		return avutil.AVERROR_ENOMEM, nil
	}
	return 0, fifo
}

func writeOutputFileHeader(outputFormatCtx *avformat.Context) int {
	ret := outputFormatCtx.AvformatWriteHeader(nil)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not write output file header (error '%s')\n", avutil.AvStrerr(ret))
		return ret
	}
	return 0
}

func writeOutputFileTrailer(outputFormatCtx *avformat.Context) int {
	ret := outputFormatCtx.AvWriteTrailer()
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
			log.Printf("[decodeAudioFrame] Read frame error %s. Set finished \n", avutil.AvStrerr(ret))
			finished = true
		} else {
			log.Printf("[decodeAudioFrame] Could not read frame (error '%s')\n", avutil.AvStrerr(ret))
			return ret, false, false
		}
	}
	ret = avcodec.AvcodecSendPacket(inputCodecCtx, packet)
	if ret < 0 {
		log.Printf("[decodeAudioFrame] Could not send packet for decoding (error '%s')\n", avutil.AvStrerr(ret))
		return ret, false, false
	}
	ret = avcodec.AvcodecReceiveFrame(inputCodecCtx, frame)
	if ret == avutil.AVERROR_EAGAIN {
		log.Printf("[decodeAudioFrame] Decode frame error %s \n", avutil.AvStrerr(ret))
		return 0, false, false
	} else if ret == avutil.AVERROR_EOF {
		log.Printf("[decodeAudioFrame] Decode frame error %s \n", avutil.AvStrerr(ret))
		return 0, false, true
	} else if ret < 0 {
		log.Printf("[decodeAudioFrame] Could not recieve frame from decoder (error '%s')\n", avutil.AvStrerr(ret))
		return ret, false, false
	} else {
		return 0, true, false
	}
}

func initConvertedSamples(convertedInputSamples ***uint8, outputCodecCtx *avcodec.Context, frameSize int) int {
	ret := avutil.AvSamplesAllocArrayAndSamples(convertedInputSamples, nil, avutil.AvGetChannelLayoutNbChannels(outputCodecCtx.ChannelLayout()), frameSize, int(outputCodecCtx.SampleFmt()), 0)
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
		log.Printf("[encodeAudioFrame] Send frame error (%s)\n", avutil.AvStrerr(ret))
		return 0, false
	} else if ret < 0 {
		log.Printf("Could not send packet for encoding (error '%s')\n", avutil.AvStrerr(ret))
		return ret, false
	}
	ret = avcodec.AvcodecReceivePacket(outputCodecCtx, outputPacket)
	if ret == avutil.AVERROR_EAGAIN {
		log.Printf("[encodeAudioFrame] recieve packet EAGAIN (%s) \n", avutil.AvStrerr(ret))
		return 0, false
	} else if ret == avutil.AVERROR_EOF {
		log.Printf("[encodeAudioFrame] recieve packet EOF (%s) \n", avutil.AvStrerr(ret))
		return 0, false
	} else if ret < 0 {
		log.Printf("Could not encode frame (error '%s')\n", avutil.AvStrerr(ret))
		return ret, false
	}
	ret = outputFormatCtx.AvWriteFrame(outputPacket)
	if ret < 0 {
		log.Printf("Could not write frame (error '%s')\n", avutil.AvStrerr(ret))
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

func flushEncoder(outputFormatCtx *avformat.Context, outputCodecCtx *avcodec.Context) {
	outputPacket := avcodec.AvPacketAlloc()
	defer avcodec.AvPacketFree(outputPacket)

	ret := avcodec.AvcodecSendFrame(outputCodecCtx, nil)
	if ret < 0 {
		return
	}
	for ret >= 0 {
		ret = avcodec.AvcodecReceivePacket(outputCodecCtx, outputPacket)
		if ret == avutil.AVERROR_EAGAIN || ret == avutil.AVERROR_EOF {
			break
		}
		ret = outputFormatCtx.AvWriteFrame(outputPacket)
	}
}

func TranscodeAudio(filename string) int {
	file_data, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	fileBuffer := bytes.NewBuffer(file_data)
	tempBuffer := make([]byte, BUF_SIZE)

	// Buffer for AVIOContext
	avioReadBuffer := (*uint8)(avutil.AvMalloc(BUF_SIZE + avcodec.AV_INPUT_BUFFER_PADDING_SIZE))
	packetBuffer := initPacketBuf(0)

	// Fill buffers
	fileBuffer.Read(tempBuffer)
	packetBuffer.WriteBuffer(tempBuffer)

	pdFilename := C.CString("")
	defer C.free(unsafe.Pointer(pdFilename))

	ret, inputFormatCtx, inputCodecCtx := openInput(packetBuffer, avioReadBuffer, BUF_SIZE, pdFilename)
	if ret < 0 {
		return ret
	}
	defer avformat.AvformatCloseInput(inputFormatCtx)
	inputCodecCtx.AvcodecFlushBuffers()

	ret, outputFormatCtx, outputCodecCtx := openOutputFile(inputCodecCtx)
	if ret < 0 {
		return ret
	}
	log.Printf("Sample rate: %d \n", outputCodecCtx.SampleRate())

	ret, fifo := initFifo(outputCodecCtx)
	if ret < 0 {
		return ret
	}

	ret, resampleCtx := initResampler(inputCodecCtx, outputCodecCtx)
	if ret < 0 {
		return ret
	}

	log.Println("Start transcoding")

	iteration := 0

	for {
		ret, octx := createCodecCtx(outputFormatCtx, inputFormatCtx.Streams()[0].CodecParameters(), inputCodecCtx.SampleRate())

		if iteration != 0 {
			read_bytes, err := fileBuffer.Read(tempBuffer)
			if err == io.EOF {
				log.Println("[transcoding] File buffer fully flushed")
				break
			}

			tempBuffer = tempBuffer[:read_bytes]
			packetBuffer.WriteBuffer(tempBuffer)
			log.Printf(
				"[transcoding] Read %d bytes from file. AVIO custom buffer length: %d. Remaining file length: %d. Temp buffer length: %d \n",
				read_bytes,
				packetBuffer.Buffer.Len(),
				fileBuffer.Len(),
				len(tempBuffer),
			)
		}

		if inputFormatCtx.Pb().EofReached() == 1 {
			log.Println("[transcoding] AVIOContext reached EOF. Flush buffer")
			// Comment line bellow to get more output frames. TODO: fix that
			avformat.AvioFeof(inputFormatCtx.Pb())
		}

		ret, outputPb := openFile(fmt.Sprintf("chunk-%d.aac", iteration), outputFormatCtx)
		if ret < 0 {
			return ret
		}
		outputFormatCtx.SetPb(outputPb)

		ret = writeOutputFileHeader(outputFormatCtx)
		if ret < 0 {
			return ret
		}

		for {
			finished := false
			outputFrameSize := octx.FrameSize()

			for fifo.AvAudioFifoSize() < outputFrameSize {
				ret, finished = readDecodeConvertAndStore(fifo, inputFormatCtx, inputCodecCtx, octx, resampleCtx)
				if ret < 0 {
					os.Exit(-ret)
				}
				if finished {
					log.Println("[transcoding] Finish decoding buffer part")
					break
				}
			}

			for fifo.AvAudioFifoSize() >= outputFrameSize ||
				(finished && fifo.AvAudioFifoSize() > 0) {
				ret = loadEncodeAndWrite(fifo, outputFormatCtx, octx)
				if ret < 0 {
					return ret
				}
			}

			if finished {
				flushEncoder(outputFormatCtx, outputCodecCtx)
				break
			}
		}

		if fileBuffer.Len() == 0 {
			log.Println("[transcoding] Flush the encoder")
			flushEncoder(outputFormatCtx, octx)
		}

		writeOutputFileTrailer(outputFormatCtx)
		avformat.AvIOClosep(&outputPb)
		inputCodecCtx.AvcodecFlushBuffers()
		avcodec.AvcodecFreeContext(octx)

		iteration += 1
	}

	fifo.AvAudioFifoFree()
	resampleCtx.SwrFree()
	avcodec.AvcodecFreeContext(outputCodecCtx)
	outputFormatCtx.AvformatFreeContext()

	return 0
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input file>\n", os.Args[0])
		os.Exit(1)
	}
	TranscodeAudio(os.Args[1])
}
