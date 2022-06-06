package muxingaudio

/*
	The original of this example is https://github.com/xtingray/audio_mixer/blob/main/audio_mixer.c
*/

import (
	"fmt"
	"log"

	"github.com/skemtoputaete/goav/avcodec"
	"github.com/skemtoputaete/goav/avfilter"
	"github.com/skemtoputaete/goav/avformat"
	"github.com/skemtoputaete/goav/avutil"
)

var output_format_context *avformat.Context
var output_codec_context *avcodec.Context
var input_format_context_0 *avformat.Context
var input_codec_context_0 *avcodec.Context
var input_format_context_1 *avformat.Context
var input_codec_context_1 *avcodec.Context

var graph *avfilter.Graph
var src0 *avfilter.Context
var src1 *avfilter.Context
var sink *avfilter.Context

func get_error_text(ret int) string {
	return avutil.AvStrerr(ret)
}

func init_filter_graph(graph **avfilter.Graph, src0 **avfilter.Context, src1 **avfilter.Context, sink **avfilter.Context) int {
	var filter_graph *avfilter.Graph
	var abuffer1_ctx *avfilter.Context
	var abuffer1 *avfilter.Filter
	var abuffer0_ctx *avfilter.Context
	var abuffer0 *avfilter.Filter
	var mix_ctx *avfilter.Context
	var mix_filter *avfilter.Filter
	var abuffersink_ctx *avfilter.Context
	var abuffersink *avfilter.Filter

	var ret int

	// Create a new filtergraph, which will contain all the filters.
	filter_graph = avfilter.AvfilterGraphAlloc()
	if filter_graph == nil {
		log.Fatal("Unable to create filter graph")
		return avutil.AVERROR_ENOMEM
	}

	/****** abuffer 0 ********/

	// Create the abuffer filter;
	// it will be used for feeding the data into the graph.
	abuffer0 = avfilter.AvfilterGetByName("abuffer")
	if abuffer0 == nil {
		log.Fatal("Could not find the abuffer filter")
		return avutil.AVERROR_FILTER_NOT_FOUND
	}

	// buffer audio source: the decoded frames from the decoder will be inserted here.
	if input_codec_context_0.ChannelLayout() == 0 {
		input_codec_context_0.SetChannelLayout(avutil.AvGetDefaultChannelLayout(input_codec_context_0.Channels()))
	}

	args := fmt.Sprintf(
		"sample_rate=%d:sample_fmt=%s:channel_layout=0x%x",
		input_codec_context_0.SampleRate(),
		avutil.AvGetSampleFmtName(int(input_codec_context_0.SampleFmt())),
		input_codec_context_0.ChannelLayout(),
	)

	ret = avfilter.AvfilterGraphCreateFilter(&abuffer0_ctx, abuffer0, "src0", args, nil, filter_graph)
	if ret < 0 {
		log.Fatal("Cannot create audio buffer source")
		return ret
	}

	// abuffer 1

	// Create the abuffer filter;
	// it will be used for feeding the data into the graph.
	abuffer1 = avfilter.AvfilterGetByName("abuffer")
	if abuffer1 == nil {
		log.Fatal("Could not find the abuffer filter")
		return avutil.AVERROR_FILTER_NOT_FOUND
	}

	// buffer audio source: the decoded frames from the decoder will be inserted here.
	if input_codec_context_1.ChannelLayout() == 0 {
		input_codec_context_1.SetChannelLayout(avutil.AvGetDefaultChannelLayout(input_codec_context_1.Channels()))
	}

	args = fmt.Sprintf(
		"sample_rate=%d:sample_fmt=%s:channel_layout=0x%x",
		input_codec_context_1.SampleRate(),
		avutil.AvGetSampleFmtName(int(input_codec_context_1.SampleFmt())),
		input_codec_context_1.ChannelLayout(),
	)

	ret = avfilter.AvfilterGraphCreateFilter(&abuffer1_ctx, abuffer1, "src1", args, nil, filter_graph)
	if ret < 0 {
		log.Fatal("Cannot create audio buffer source")
		return ret
	}

	// amix
	// Create mix filter.
	mix_filter = avfilter.AvfilterGetByName("amix")
	if mix_filter == nil {
		log.Fatal("Could not find the mix filter")
		return avutil.AVERROR_FILTER_NOT_FOUND
	}

	args = "inputs=2"

	ret = avfilter.AvfilterGraphCreateFilter(&mix_ctx, mix_filter, "amix", args, nil, filter_graph)
	if ret < 0 {
		log.Fatal("Cannot create audio amix filter")
		return ret
	}

	// Finally create the abuffersink filter;
	// it will be used to get the filtered data out of the graph.

	abuffersink = avfilter.AvfilterGetByName("abuffersink")
	if abuffersink == nil {
		log.Fatal("Could not find the abuffersink filter")
		return avutil.AVERROR_FILTER_NOT_FOUND
	}

	abuffersink_ctx = filter_graph.AvfilterGraphAllocFilter(abuffersink, "sink")
	if abuffersink_ctx == nil {
		log.Fatal("Could not allocate the abuffersink instance")
		return avutil.AVERROR_ENOMEM
	}

	// Same sample fmts as the output file.
	val := []int{avutil.AV_SAMPLE_FMT_S16, avutil.AV_SAMPLE_FMT_NONE}
	ret = avutil.AvOptSetIntList(abuffersink_ctx, "sample_fmts", val, avutil.AV_SAMPLE_FMT_NONE, avutil.AV_OPT_SEARCH_CHILDREN)

	ch_layout := avutil.AvGetChannelLayoutString(0, 2)
	avutil.AvOptSet(abuffersink_ctx, "channel_layout", ch_layout, avutil.AV_OPT_SEARCH_CHILDREN)

	if ret < 0 {
		log.Fatal("Could set options to the abuffersink instance")
		return ret
	}

	ret = abuffersink_ctx.AvfilterInitStr("")
	if ret < 0 {
		log.Fatal("Could not initialize the abuffersink instance")
		return ret
	}

	// Connect the filters

	ret = avfilter.AvfilterLink(abuffer0_ctx, 0, mix_ctx, 0)
	if ret >= 0 {
		ret = avfilter.AvfilterLink(abuffer1_ctx, 0, mix_ctx, 1)
	}
	if ret >= 0 {
		ret = avfilter.AvfilterLink(mix_ctx, 0, abuffersink_ctx, 0)
	}
	if ret < 0 {
		log.Fatal("Error connecting filters")
		return ret
	}

	// Configure the graph.
	ret = filter_graph.AvfilterGraphConfig(nil)
	if ret < 0 {
		log.Fatal("Error while configuring graph : %s\n", get_error_text(ret))
		return ret
	}

	dump := filter_graph.AvfilterGraphDump("")
	log.Printf("Graph :\n%s\n", dump)

	*graph = filter_graph
	*src0 = abuffer0_ctx
	*src1 = abuffer1_ctx
	*sink = abuffersink_ctx

	return 0
}

func main() {

}
