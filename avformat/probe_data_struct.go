package avformat

//#cgo pkg-config: libavformat libavcodec libavutil libavdevice libavfilter libswresample libswscale
//#include <libavformat/avformat.h>
import "C"
import (
	"unsafe"
)

func (pd *AvProbeData) SetBuf(buf *uint8) {
	pd.buf = (*C.uchar)(unsafe.Pointer(buf))
}

func (pd *AvProbeData) SetBufSize(buf_size int) {
	pd.buf_size = C.int(buf_size)
}

func (pd *AvProbeData) SetFilename(filename *CString) {
	pd.filename = (*C.char)(filename)
}
