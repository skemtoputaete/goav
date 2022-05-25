package main

import (
	"fmt"
	"log"
	"os"

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
)

func openInputFile(filename string) (int, *avformat.Context, *avcodec.Context) {
	ret := 0
	// AVFormatContext содержит данные из заголовка контейнера
	ifmtCtx := avformat.AvformatAllocContext()
	if avformat.AvformatOpenInput(&ifmtCtx, filename, nil, nil) != 0 {
		fmt.Fprintf(os.Stderr, "Unable to open file %s\n", filename)
		return avutil.AVERROR_EXIT, nil, nil
	}
	defer func() {
		if ret < 0 {
			avformat.AvformatCloseInput(ifmtCtx)
		}
	}()

	if ifmtCtx.AvformatFindStreamInfo(nil) < 0 {
		fmt.Fprintf(os.Stderr, "Couldn't find stream information")
		ret = avutil.AVERROR_EXIT
		return ret, nil, nil
	}
	ifmtCtx.AvDumpFormat(0, filename, 0)

	if ifmtCtx.NbStreams() != 1 {
		fmt.Fprintf(os.Stderr, "Expected one audio input stream, but found %d\n", ifmtCtx.NbStreams())
		ret = avutil.AVERROR_EXIT
		return ret, nil, nil
	}

	codecId := ifmtCtx.Streams()[0].CodecParameters().CodecId()
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
	ret = avcodec.AvcodecParametersToContext(avCtx, ifmtCtx.Streams()[0].CodecParameters())
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not get codec parameters\n")
		return ret, nil, nil
	}
	if ret = avCtx.AvcodecOpen2(inputCodec, nil); ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not open input codec (error '%s')\n", avutil.AvStrerr(ret))
		return ret, nil, nil
	}

	return 0, ifmtCtx, avCtx
}

func openOutputFile(filename string, inputCodecCtx *avcodec.Context) (int, *avformat.Context, *avcodec.Context) {
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

func initInputFrame() (int, *avutil.Frame) {
	frame := avutil.AvFrameAlloc()
	if frame == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate input frame\n")
		return avutil.AVERROR_ENOMEM, nil
	}
	return 0, frame
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

/* Global timestamp for the audio frames. */
var pts int64

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

func writeOutputFileTrailer(outputFormatCtx *avformat.Context) int {
	ret := outputFormatCtx.AvWriteTrailer()
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not write output file trailer (error '%s')\n", avutil.AvStrerr(ret))
		return ret
	}
	return 0
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

func TranscodeAudio(infile, outfile string) int {
	ret, inputFormatCtx, inputCodecCtx := openInputFile(infile)
	if ret < 0 {
		return ret
	}
	defer avformat.AvformatCloseInput(inputFormatCtx)
	defer avcodec.AvcodecFreeContext(inputCodecCtx)

	ret, outputFormatCtx, outputCodecCtx := openOutputFile(outfile, inputCodecCtx)
	if ret < 0 {
		return ret
	}
	defer func() {
		avcodec.AvcodecFreeContext(outputCodecCtx)
		pb := outputFormatCtx.Pb()
		avformat.AvIOClosep(&pb)
		outputFormatCtx.AvformatFreeContext()
	}()

	ret, resampleCtx := initResampler(inputCodecCtx, outputCodecCtx)
	if ret < 0 {
		return ret
	}
	defer resampleCtx.SwrFree()

	ret, fifo := initFifo(outputCodecCtx)
	if ret < 0 {
		return ret
	}
	defer fifo.AvAudioFifoFree()

	// ret = writeOutputFileHeader(outputFormatCtx)
	// if ret < 0 {
	// 	return ret
	// }
	// defer writeOutputFileTrailer(outputFormatCtx)

	iteration := 0
	ret, outputPb := openFile(fmt.Sprintf("chunk-%d.aac", iteration), outputFormatCtx)
	outputFormatCtx.SetPb(outputPb)
	ret = writeOutputFileHeader(outputFormatCtx)

	for {
		if iteration%100 == 0 {
			writeOutputFileTrailer(outputFormatCtx)
			avformat.AvIOClosep(&outputPb)
			ret, outputPb := openFile(fmt.Sprintf("chunk-%d.aac", iteration), outputFormatCtx)
			outputFormatCtx.SetPb(outputPb)
			ret = writeOutputFileHeader(outputFormatCtx)

			if ret < 0 {
				return ret
			}
		}

		finished := false
		outputFrameSize := outputCodecCtx.FrameSize()
		log.Printf("Output frame size %d \n", outputFrameSize)
		for fifo.AvAudioFifoSize() < outputFrameSize {
			ret, finished = readDecodeConvertAndStore(fifo, inputFormatCtx, inputCodecCtx, outputCodecCtx, resampleCtx)
			if ret < 0 {
				os.Exit(-ret)
			}
			if finished {
				break
			}
		}
		for fifo.AvAudioFifoSize() >= outputFrameSize ||
			(finished && fifo.AvAudioFifoSize() > 0) {
			ret = loadEncodeAndWrite(fifo, outputFormatCtx, outputCodecCtx)
			if ret < 0 {
				return ret
			}
		}
		if finished {
			dataWritten := true
			for dataWritten {
				ret, dataWritten = encodeAudioFrame(nil, outputFormatCtx, outputCodecCtx)
				if ret < 0 {
					return ret
				}
			}
			break
		}
		iteration += 1
	}

	writeOutputFileTrailer(outputFormatCtx)
	avformat.AvIOClosep(&outputPb)
	return 0
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input file> <output file>\n", os.Args[0])
		os.Exit(1)
	}
	os.Exit(-TranscodeAudio(os.Args[1], os.Args[2]))
}
