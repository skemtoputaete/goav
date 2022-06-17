package main

//#cgo pkg-config: libavutil
//#include <libavutil/frame.h>
//#include <libavutil/channel_layout.h>
//#include <stdlib.h>
import "C"

import (
	"fmt"
	"os"

	"github.com/skemtoputaete/goav/avcodec"
	"github.com/skemtoputaete/goav/avfilter"
	"github.com/skemtoputaete/goav/avformat"
	"github.com/skemtoputaete/goav/avutil"
)

type InputStream struct {
	StreamIndex  int
	DecodeFrame  *avutil.Frame
	DecodePacket *avcodec.Packet
	FormatCtx    *avformat.Context
	DecoderCtx   *avcodec.Context
	BufferSrc    *avfilter.Context
	BufferSink   *avfilter.Context
	Graph        *avfilter.Graph
	EofReached   bool
	Closed       bool
}

func (ist *InputStream) Close() {
	if ist.Closed {
		return
	}
	ist.Graph.AvBuffersrcAddFrameFlags(ist.BufferSrc, nil, avfilter.AV_BUFFERSRC_FLAG_PUSH)
	ist.Closed = true
}

type OutputStream struct {
	EncodePacket  *avcodec.Packet
	FilteredFrame *avutil.Frame
	FormatCtx     *avformat.Context
	EncoderCtx    *avcodec.Context
	BufferSink    *avfilter.Context
	Graph         *avfilter.Graph
}

func (ost *OutputStream) Close() {

}

func OpenInputFile(filename string) (int, *InputStream) {
	ret := 0
	ist := &InputStream{}
	defer func() {
		if ret < 0 {
			ist = nil
		}
	}()

	ifmtCtx := avformat.AvformatAllocContext()
	if avformat.AvformatOpenInput(&ifmtCtx, filename, nil, nil) != 0 {
		fmt.Fprintf(os.Stderr, "Unable to open file %s\n", filename)
		return avutil.AVERROR_EXIT, nil
	}
	defer func() {
		if ret < 0 {
			avformat.AvformatCloseInput(ifmtCtx)
		}
	}()

	if ifmtCtx.AvformatFindStreamInfo(nil) < 0 {
		fmt.Fprintf(os.Stderr, "Couldn't find stream information")
		ret = avutil.AVERROR_EXIT
		return ret, nil
	}
	ist.FormatCtx = ifmtCtx
	ifmtCtx.AvDumpFormat(0, filename, 0)

	var dec *avcodec.Codec
	ret = avformat.AvFindBestStream(ifmtCtx, avcodec.AVMEDIA_TYPE_AUDIO, -1, 0, &dec, 0)
	if ret < 0 {
		return ret, nil
	}
	ist.StreamIndex = ret
	stream := ifmtCtx.Streams()[ist.StreamIndex]

	codecCtx := dec.AvcodecAllocContext3()
	if codecCtx == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate a decoding context\n")
		ret = avutil.AVERROR_ENOMEM
		return ret, nil
	}
	defer func() {
		if ret < 0 {
			avcodec.AvcodecFreeContext(codecCtx)
		}
	}()
	ist.DecoderCtx = codecCtx

	ret = avcodec.AvcodecParametersToContext(codecCtx, stream.CodecParameters())
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not get codec parameters\n")
		return ret, nil
	}

	if ret = codecCtx.AvcodecOpen2(dec, nil); ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not open input codec (error '%s')\n", avutil.AvStrerr(ret))
		return ret, nil
	}

	ist.DecodeFrame = avutil.AvFrameAlloc()
	if ist.DecodeFrame == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate input frame\n")
		return avutil.AVERROR_ENOMEM, nil
	}
	ist.DecodePacket = avcodec.AvPacketAlloc()
	if ist.DecodePacket == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate input packet\n")
		return avutil.AVERROR_ENOMEM, nil
	}

	return 0, ist
}

func OpenOutputFile(filename string, bufferSink *avfilter.Context) (int, *OutputStream) {
	ret := 0
	ost := &OutputStream{}
	defer func() {
		ost = nil
	}()

	ofmtCtx := avformat.AvformatAllocContext()
	if avformat.AvformatAllocOutputContext2(&ofmtCtx, nil, "", filename) != 0 {
		fmt.Fprintf(os.Stderr, "Unable to alloc output context for %s\n", filename)
		ret = avutil.AVERROR_ENOMEM
		return ret, nil
	}
	defer func() {
		if ret < 0 {
			ofmtCtx.AvformatFreeContext()
		}
	}()
	ost.FormatCtx = ofmtCtx

	pb := (*avformat.AvIOContext)(nil)
	ret = avformat.AvIOOpen(&pb, filename, avformat.AVIO_FLAG_WRITE)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not open output file '%s'\n", filename)
		ret = avutil.AVERROR_EXIT
		return ret, nil
	}
	ofmtCtx.SetPb(pb)
	defer func() {
		if ret < 0 {
			pb := ofmtCtx.Pb()
			avformat.AvIOClosep(&pb)
		}
	}()

	outStream := ofmtCtx.AvformatNewStream(nil)
	if outStream == nil {
		fmt.Fprintf(os.Stderr, "Could not reate new stream\n")
		ret = avutil.AVERROR_ENOMEM
		return ret, nil
	}

	outputCodec := avcodec.AvcodecFindEncoder(avcodec.CodecId(avcodec.AV_CODEC_ID_AAC))
	if outputCodec == nil {
		fmt.Fprintf(os.Stderr, "Could not find an AAC encoder\n")
		ret = avutil.AVERROR_EXIT
		return ret, nil
	}

	codecCtx := outputCodec.AvcodecAllocContext3()
	if codecCtx == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate an encoding context\n")
		ret = avutil.AVERROR_ENOMEM
		return ret, nil
	}
	defer func() {
		if ret < 0 {
			avcodec.AvcodecFreeContext(codecCtx)
		}
	}()
	ost.EncoderCtx = codecCtx

	/* Allow the use of the experimental AAC encoder. */
	codecCtx.SetStrictStdCompliance(avcodec.FF_COMPLIANCE_EXPERIMENTAL)

	/* Set the basic encoder parameters.
	 * The input file's sample rate is used to avoid a sample rate conversion. */
	codecCtx.SetChannels(bufferSink.GetChannels())
	codecCtx.SetChannelLayout(uint64(bufferSink.GetChannelLayout()))
	codecCtx.SetSampleRate(bufferSink.GetSampleRate())
	codecCtx.SetSampleFmt(avcodec.AvSampleFormat(bufferSink.GetFormat()))
	/* Set the sample rate for the container. */
	codecCtx.SetTimeBase(avutil.NewRational(1, codecCtx.SampleRate()))

	if ret = codecCtx.AvcodecOpen2(outputCodec, nil); ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not open output codec (error '%s')\n", avutil.AvStrerr(ret))
		return ret, nil
	}
	ret = avcodec.AvcodecParametersFromContext(outStream.CodecParameters(), codecCtx)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not initialize stream parameters\n")
		return ret, nil
	}
	outStream.SetTimeBase(codecCtx.TimeBase())

	/* Some container formats (like MP4) require global headers to be present.
	 * Mark the encoder so that it behaves accordingly. */
	if (ofmtCtx.Oformat().Flags() & avformat.AVFMT_GLOBALHEADER) != 0 {
		codecCtx.SetFlags(codecCtx.Flags() | avcodec.AV_CODEC_FLAG_GLOBAL_HEADER)
	}

	ost.FilteredFrame = avutil.AvFrameAlloc()
	if ost.FilteredFrame == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate input frame\n")
		return avutil.AVERROR_ENOMEM, nil
	}
	ost.EncodePacket = avcodec.AvPacketAlloc()
	if ost.EncodePacket == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate input packet\n")
		return avutil.AVERROR_ENOMEM, nil
	}

	return 0, ost
}

func InitGraph(ist1, ist2 *InputStream) (int, *avfilter.Graph) {
	ret := 0

	input1_cfg := fmt.Sprintf(
		"abuffer@in1=time_base=%d/%d:sample_rate=%d:sample_fmt=%s:channel_layout=0x%x",
		ist1.DecoderCtx.TimeBase().Num(),
		ist1.DecoderCtx.TimeBase().Den(),
		ist1.DecoderCtx.SampleRate(),
		avutil.AvGetSampleFmtName(int(ist1.DecoderCtx.SampleFmt())),
		ist1.DecoderCtx.ChannelLayout(),
	)

	input2_cfg := fmt.Sprintf(
		"abuffer@in2=time_base=%d/%d:sample_rate=%d:sample_fmt=%s:channel_layout=0x%x",
		ist2.DecoderCtx.TimeBase().Num(),
		ist2.DecoderCtx.TimeBase().Den(),
		ist2.DecoderCtx.SampleRate(),
		avutil.AvGetSampleFmtName(int(ist2.DecoderCtx.SampleFmt())),
		ist2.DecoderCtx.ChannelLayout(),
	)

	if ist1.DecoderCtx.SampleRate() < ist2.DecoderCtx.SampleRate() {
		input1_cfg = fmt.Sprintf(
			"%s [resample_out]; [resample_out] aresample=%d [in_1]",
			input1_cfg,
			ist2.DecoderCtx.SampleRate(),
		)
		input2_cfg = fmt.Sprintf(
			"%s [in_2]",
			input2_cfg,
		)
	} else if ist1.DecoderCtx.SampleRate() > ist2.DecoderCtx.SampleRate() {
		input2_cfg = fmt.Sprintf(
			"%s [resample_out]; [resample_out] aresample@resampler=%d [in_2]",
			input2_cfg,
			ist1.DecoderCtx.SampleRate(),
		)
		input1_cfg = fmt.Sprintf(
			"%s [in_1]",
			input1_cfg,
		)
	} else {
		input1_cfg = fmt.Sprintf(
			"%s [in_1]",
			input1_cfg,
		)
		input2_cfg = fmt.Sprintf(
			"%s [in_2]",
			input2_cfg,
		)
	}

	graph_cfg := fmt.Sprintf(
		"%s; %s; [in_1] [in_2] amix@mix=inputs=2:duration=longest [result]; [result] abuffersink@result",
		input1_cfg,
		input2_cfg,
	)

	graph := avfilter.AvfilterGraphAlloc()
	if graph == nil {
		fmt.Fprintf(os.Stderr, "Could not allocate input packet\n")
		return avutil.AVERROR_ENOMEM, nil
	}
	graph.SetNbThreads(1)

	var inputs *avfilter.Input
	var outputs *avfilter.Input
	defer func() {
		avfilter.AvfilterInoutFree(&inputs)
		avfilter.AvfilterInoutFree(&outputs)
	}()

	ret = graph.AvfilterGraphParse2(graph_cfg, &inputs, &outputs)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not parse graph config (%s) \n", graph_cfg)
		return ret, nil
	}

	ret = graph.AvfilterGraphConfig(nil)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Can not configure graph \n")
		return ret, nil
	}

	return 0, graph
}

func (ist *InputStream) Decode() int {
	ret := 0

	for {
		avcodec.AvPacketUnref(ist.DecodePacket)
		ret = ist.FormatCtx.AvReadFrame(ist.DecodePacket)
		if ret < 0 {
			fmt.Fprintf(os.Stderr, "Could not read frame. Error: %s \n", avutil.AvStrerr(ret))
			return ret
		}
		if ist.DecodePacket.StreamIndex() == ist.StreamIndex {
			break
		}
		// fmt.Fprintf(os.Stderr, "Read frame from another stream. Another try... \n")
	}

	ret = avcodec.AvcodecSendPacket(ist.DecoderCtx, ist.DecodePacket)
	if ret < 0 {
		fmt.Fprintf(os.Stderr, "Could not send packet for decoding. Error: %s \n", avutil.AvStrerr(ret))
		return ret
	}

	for {
		/* Receive one frame from the decoder. */
		ret = avcodec.AvcodecReceiveFrame(ist.DecoderCtx, ist.DecodeFrame)
		if ret == avutil.AVERROR_EOF || ret == avutil.AVERROR_EAGAIN {
			ret = 0
			break
		} else if ret < 0 {
			fmt.Fprintf(os.Stderr, "Could not decode frame. Error: %s \n", avutil.AvStrerr(ret))
			return ret
		}
		ist.DecodeFrame.SetPts(ist.DecodeFrame.BestEffortTimestamp())
		// fmt.Fprintf(os.Stdin, "Pushing decoded frame to filters\n")
		/* Push the decoded frame into the filtergraph */
		ret = ist.Graph.AvBuffersrcAddFrameFlags(ist.BufferSrc, ist.DecodeFrame, avfilter.AV_BUFFERSRC_FLAG_PUSH)
		if ret < 0 {
			fmt.Fprintf(os.Stderr, "Error while feeding the filtergraph: %s \n", avutil.AvStrerr(ret))
			return ret
		}
	}

	return ret
}

func (ost *OutputStream) Encode() int {
	ret := 0

	/* pull filtered frames from the filtergraph */
	for ret >= 0 {
		// fmt.Fprintf(os.Stdin, "Pulling filtered frame from filters \n")
		avutil.AvFrameUnref(ost.FilteredFrame)
		ret = ost.Graph.AvBuffersinkGetFrame(ost.BufferSink, ost.FilteredFrame)
		if ret < 0 {
			if ret == avutil.AVERROR_EAGAIN || ret == avutil.AVERROR_EOF {
				ret = 0
			} else {
				fmt.Fprintf(os.Stderr, "Error while get frame from filtergraph: %s \n", avutil.AvStrerr(ret))
			}
			break
		}
		ret = ost.WriteFrame(false)
	}

	return ret
}

func (ost *OutputStream) WriteFrame(flush bool) int {
	frame := ost.FilteredFrame
	if flush {
		frame = nil
	}

	ret := 0

	// fmt.Fprintf(os.Stdin, "Encoding frame\n")
	/* encode filtered frame */
	avcodec.AvPacketUnref(ost.EncodePacket)
	ret = avcodec.AvcodecSendFrame(ost.EncoderCtx, frame)
	if ret < 0 {
		return ret
	}

	for ret >= 0 {
		ret = avcodec.AvcodecReceivePacket(ost.EncoderCtx, ost.EncodePacket)
		if ret == avutil.AVERROR_EAGAIN || ret == avutil.AVERROR_EOF {
			return 0
		}

		/* TODO: prepare packet for muxing */
		/* mux encoded frame */
		ret = ost.FormatCtx.AvInterleavedWriteFrame(ost.EncodePacket)
	}

	return ret
}

func ProcessInputs(inputs []*InputStream, ost *OutputStream) int {
	ret := 0
	finishedCount := 0

	for {
		if finishedCount == len(inputs) {
			break
		}

		for _, input := range inputs {
			if input.EofReached {
				continue
			}

			ret = input.Decode()
			if ret == avutil.AVERROR_EOF {
				input.EofReached = true
				finishedCount++
				input.Close()
			} else if ret == avutil.AVERROR_EAGAIN {
				continue
			} else if ret < 0 {
				break
			}
		}

		ret = ost.Encode()
		if ret == avutil.AVERROR_EOF || ret == avutil.AVERROR_EAGAIN {
			ret = 0
		}
		if ret < 0 {
			break
		}
	}

	return ret
}

func TranscodeAudio(filename1, filename2, output string) int {
	ret := 0

	ret, ist1 := OpenInputFile(filename1)
	defer ist1.Close()
	if ret < 0 {
		return ret
	}
	ret, ist2 := OpenInputFile(filename2)
	defer ist2.Close()
	if ret < 0 {
		return ret
	}
	ret, graph := InitGraph(ist1, ist2)
	defer graph.AvfilterGraphFree()
	if ret < 0 {
		return ret
	}

	fmt.Println(graph.AvfilterGraphDump(""))

	ist1.Graph = graph
	ist2.Graph = graph
	ist1.BufferSrc = graph.AvfilterGraphGetFilter("abuffer@in1")
	ist2.BufferSrc = graph.AvfilterGraphGetFilter("abuffer@in2")
	bufferSink := graph.AvfilterGraphGetFilter("abuffersink@result")

	ret, ost := OpenOutputFile(output, bufferSink)
	defer ost.Close()
	ost.BufferSink = bufferSink
	ret = ost.FormatCtx.AvformatWriteHeader(nil)
	if ret < 0 {
		return ret
	}

	inputs := []*InputStream{ist1, ist2}
	ret = ProcessInputs(inputs, ost)
	if ret < 0 {
		return ret
	}

	ret = ost.FormatCtx.AvWriteTrailer()
	return ret
}

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input file 1> <input file 2> <output file>\n", os.Args[0])
		os.Exit(1)
	}
	os.Exit(-TranscodeAudio(os.Args[1], os.Args[2], os.Args[3]))
}
