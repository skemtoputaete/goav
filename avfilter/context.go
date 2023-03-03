package avfilter

//#cgo pkg-config: libavfilter
//#include <libavfilter/avfilter.h>
//#include <libavfilter/buffersrc.h>
//#include <libavfilter/buffersink.h>
import "C"
import (
	"unsafe"

	"github.com/skemtoputaete/goav/avutil"
)

func (c *Context) AvBuffersrcWriteFrame(frame *avutil.Frame) int {
	return int(C.av_buffersrc_write_frame((*C.struct_AVFilterContext)(c), (*C.struct_AVFrame)(unsafe.Pointer(frame))))
}

func (c *Context) AvBuffersinkGetTimeBase() avutil.Rational {
	time_base := C.av_buffersink_get_time_base((*C.struct_AVFilterContext)(c))
	return *(*avutil.Rational)(unsafe.Pointer(&time_base))
}
