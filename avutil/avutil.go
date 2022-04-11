// Use of this source code is governed by a MIT license that can be found in the LICENSE file.
// Giorgis (habtom@giorgis.io)

// Package avutil is a utility library to aid portable multimedia programming.
// It contains safe portable string functions, random number generators, data structures,
// additional mathematics functions, cryptography and multimedia related functionality.
// Some generic features and utilities provided by the libavutil library
package avutil

//#cgo pkg-config: libavutil
//#include <libavutil/avutil.h>
//#include <libavutil/audio_fifo.h>
//#include <libavutil/channel_layout.h>
//#include <libavutil/pixdesc.h>
//#include <stdlib.h>
//#include <errno.h>
import "C"
import (
	"fmt"
	"unsafe"
)

type (
	Options        C.struct_AVOptions
	AvTree         C.struct_AVTree
	Rational       C.struct_AVRational
	MediaType      C.enum_AVMediaType
	AvPictureType  C.enum_AVPictureType
	PixelFormat    C.enum_AVPixelFormat
	AvSampleFormat C.enum_AVSampleFormat
	File           C.FILE
	AvAudioFifo    C.struct_AVAudioFifo
)

const (
	AV_TIME_BASE   = C.AV_TIME_BASE
	AV_NOPTS_VALUE = C.AV_NOPTS_VALUE
)

var AV_TIME_BASE_Q Rational = NewRational(1, AV_TIME_BASE)

const (
	AVMEDIA_TYPE_UNKNOWN    = C.AVMEDIA_TYPE_UNKNOWN
	AVMEDIA_TYPE_VIDEO      = C.AVMEDIA_TYPE_VIDEO
	AVMEDIA_TYPE_AUDIO      = C.AVMEDIA_TYPE_AUDIO
	AVMEDIA_TYPE_DATA       = C.AVMEDIA_TYPE_DATA
	AVMEDIA_TYPE_SUBTITLE   = C.AVMEDIA_TYPE_SUBTITLE
	AVMEDIA_TYPE_ATTACHMENT = C.AVMEDIA_TYPE_ATTACHMENT
	AVMEDIA_TYPE_NB         = C.AVMEDIA_TYPE_NB
	AVERROR_EXIT            = C.AVERROR_EXIT
	AVERROR_ENOMEM          = -12
)

// MediaTypeFromString returns a media type from a string
func MediaTypeFromString(i string) MediaType {
	switch i {
	case "audio":
		return AVMEDIA_TYPE_AUDIO
	case "subtitle":
		return AVMEDIA_TYPE_SUBTITLE
	case "video":
		return AVMEDIA_TYPE_VIDEO
	default:
		return -1
	}
}

const (
	AV_CH_FRONT_LEFT    = 0x1
	AV_CH_FRONT_RIGHT   = 0x2
	AV_CH_LAYOUT_STEREO = 0x3 //(AV_CH_FRONT_LEFT | AV_CH_FRONT_RIGHT)
)

const (
	AVERROR_EAGAIN    = -(C.EAGAIN)
	AVERROR_EIO       = -(C.EIO)
	AVERROR_EOF       = C.AVERROR_EOF
	AVERROR_EPERM     = -(C.EPERM)
	AVERROR_EPIPE     = -(C.EPIPE)
	AVERROR_ETIMEDOUT = -(C.ETIMEDOUT)
)

const (
	MAX_AVERROR_STR_LEN        = 255
	MAX_CHANNEL_LAYOUT_STR_LEN = 64
)

const (
	AV_PICTURE_TYPE_NONE = C.AV_PICTURE_TYPE_NONE
	AV_PICTURE_TYPE_I    = C.AV_PICTURE_TYPE_I
	AV_PICTURE_TYPE_B    = C.AV_PICTURE_TYPE_B
	AV_PICTURE_TYPE_P    = C.AV_PICTURE_TYPE_P
)

//Return the LIBAvUTIL_VERSION_INT constant.
func AvutilVersion() uint {
	return uint(C.avutil_version())
}

//Return the libavutil build-time configuration.
func AvutilConfiguration() string {
	return C.GoString(C.avutil_configuration())
}

//Return the libavutil license.
func AvutilLicense() string {
	return C.GoString(C.avutil_license())
}

//Return a string describing the media_type enum, NULL if media_type is unknown.
func AvGetMediaTypeString(mt MediaType) string {
	return C.GoString(C.av_get_media_type_string((C.enum_AVMediaType)(mt)))
}

func AvAudioFifoAlloc(sampleFmt AvSampleFormat, channels int, nbSamples int) *AvAudioFifo {
	avAudioFifo := C.av_audio_fifo_alloc((C.enum_AVSampleFormat)(sampleFmt), C.int(channels), C.int(nbSamples))
	return (*AvAudioFifo)(avAudioFifo)
}

func AvGetDefaultChannelLayout(channels int) uint64 {
	return uint64(C.av_get_default_channel_layout(C.int(channels)))
}

//Return a single letter to describe the given picture type pict_type.
func AvGetPictureTypeChar(pt AvPictureType) string {
	return string(C.av_get_picture_type_char((C.enum_AVPictureType)(pt)))
}

//Return x default pointer in case p is NULL.
func AvXIfNull(p, x int) {
	C.av_x_if_null(unsafe.Pointer(&p), unsafe.Pointer(&x))
}

//Compute the length of an integer list.
func AvIntListLengthForSize(e uint, l int, t uint64) uint {
	return uint(C.av_int_list_length_for_size(C.uint(e), unsafe.Pointer(&l), (C.uint64_t)(t)))
}

//Open a file using a UTF-8 filename.
func AvFopenUtf8(p, m string) *File {
	cp := C.CString(p)
	defer C.free(unsafe.Pointer(cp))
	cm := C.CString(m)
	defer C.free(unsafe.Pointer(cm))
	f := C.av_fopen_utf8(cp, cm)
	return (*File)(f)
}

//Return the fractional representation of the internal time base.
func AvGetTimeBaseQ() Rational {
	return (Rational)(C.av_get_time_base_q())
}

func AvGetChannelLayoutNbChannels(channelLayout uint64) int {
	return int(C.av_get_channel_layout_nb_channels(C.uint64_t(channelLayout)))
}

func AvGetPixFmtName(pixFmt PixelFormat) string {
	s := C.av_get_pix_fmt_name((C.enum_AVPixelFormat)(pixFmt))
	if s == nil {
		return fmt.Sprintf("unknown pixel format with value %d", pixFmt)
	}
	return C.GoString(s)
}

func AvGetChannelLayoutString(nbChannels int, channelLayout uint64) string {
	bufSize := C.size_t(MAX_CHANNEL_LAYOUT_STR_LEN)
	buf := (*C.char)(C.malloc(bufSize))
	if buf == nil {
		return fmt.Sprintf("unknown channel layout with code %d", channelLayout)
	}
	defer C.free(unsafe.Pointer(buf))
	C.av_get_channel_layout_string(buf, C.int(bufSize), C.int(nbChannels), C.uint64_t(channelLayout))
	return C.GoString(buf)
}

func AvStrerr(errcode int) string {
	errbufSize := C.size_t(MAX_AVERROR_STR_LEN)
	errbuf := (*C.char)(C.malloc(errbufSize))
	if errbuf == nil {
		return fmt.Sprintf("unknown error with code %d", errcode)
	}
	defer C.free(unsafe.Pointer(errbuf))
	ret := C.av_strerror(C.int(errcode), errbuf, errbufSize)
	if ret < 0 {
		return fmt.Sprintf("unknown error with code %d", errcode)
	}
	return C.GoString(errbuf)
}
