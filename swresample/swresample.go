// Use of this source code is governed by a MIT license that can be found in the LICENSE file.
// Giorgis (habtom@giorgis.io)

// Package swresample provides a high-level interface to the libswresample library audio resampling utilities
// The process of changing the sampling rate of a discrete signal to obtain a new discrete representation of the underlying continuous signal.
package swresample

/*
	#cgo pkg-config: libswresample
	#include <libswresample/swresample.h>
*/
import "C"

type (
	Context        C.struct_SwrContext
	Frame          C.struct_AVFrame
	Class          C.struct_AVClass
	AvSampleFormat C.enum_AVSampleFormat
)

//Get the Class for Context.
func SwrGetClass() *Class {
	return (*Class)(C.swr_get_class())
}

//Context constructor functions.Allocate Context.
func SwrAlloc() *Context {
	return (*Context)(C.swr_alloc())
}

//Configuration accessors
func SwresampleVersion() uint {
	return uint(C.swresample_version())
}

func SwresampleConfiguration() string {
	return C.GoString(C.swresample_configuration())
}

func SwresampleLicense() string {
	return C.GoString(C.swresample_license())
}

func SwrAllocSetOpts(out_ch_layout int64, out_sample_fmt AvSampleFormat, out_sample_rate int,
	in_ch_layout int64, in_sample_fmt AvSampleFormat, in_sample_rate int) *Context {

	return (*Context)(C.swr_alloc_set_opts(nil, C.int64_t(out_ch_layout), C.enum_AVSampleFormat(out_sample_fmt), C.int(out_sample_rate), C.int64_t(in_ch_layout), C.enum_AVSampleFormat(in_sample_fmt), C.int(in_sample_rate), C.int(0), nil))
}
