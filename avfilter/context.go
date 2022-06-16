package avfilter

//#cgo pkg-config: libavfilter
//#include <libavfilter/avfilter.h>
//#include <libavfilter/buffersrc.h>
import "C"
import (
	"unsafe"

	"github.com/skemtoputaete/goav/avutil"
)

func (c *Context) AvBuffersrcWriteFrame(frame *avutil.Frame) int {
	return int(C.av_buffersrc_write_frame((*C.struct_AVFilterContext)(c), (*C.struct_AVFrame)(unsafe.Pointer(frame))))
}
