package avformat

//#cgo pkg-config: libavformat libavcodec libavutil libavdevice libavfilter libswresample libswscale
//#include <stdio.h>
//#include <stdlib.h>
//#include <libavformat/avformat.h>
//#include <libavformat/avio.h>
//extern int read_buffer_cb(void*, uint8_t*, int);
//extern int write_buffer_cb(void*, uint8_t*, int);
//extern int64_t seekCallBack(void*, int64_t, int);
import "C"
import (
	"bytes"
	"unsafe"

	"github.com/skemtoputaete/goav/avutil"
)

type AvioCustomBuffer struct {
	Buffer *bytes.Buffer
}

var ContextBufferMap = make(map[*Context]*AvioCustomBuffer)

func (custom_buf *AvioCustomBuffer) ReadBuffer(n_bytes int) ([]byte, int, int) {
	result := make([]byte, n_bytes)
	read_bytes, err := custom_buf.Buffer.Read(result)

	if err != nil {
		return nil, 0, avutil.AVERROR_EOF
	}

	return result, read_bytes, 0
}

func (custom_buf *AvioCustomBuffer) WriteBuffer(data []byte) int {
	write_bytes, err := custom_buf.Buffer.Write(data)

	if err != nil {
		panic("Can't write data to buffer")
	}

	return write_bytes
}

func AvioAllocContext(fmt_ctx *Context, custom_buf *AvioCustomBuffer, buf *uint8, buf_size int, write_flag int) *AvIOContext {
	avio_context := (*AvIOContext)(
		C.avio_alloc_context(
			(*C.uchar)(unsafe.Pointer(buf)), // Pointer to buffer
			C.int(buf_size),                 // Size of buffer
			C.int(write_flag),               // Buffer userd for writing
			unsafe.Pointer(fmt_ctx),         // Custom user specified data
			(*[0]byte)(C.read_buffer_cb),    // Function for reading packets
			nil,                             // Function for writing packets
			nil,                             // Function for seeking
		),
	)
	ContextBufferMap[fmt_ctx] = custom_buf

	return avio_context
}

func AvioContextFree(avio_ctx **AvIOContext) {
	bfr_ptr := (*avio_ctx).buffer

	avutil.AvFreep(unsafe.Pointer(bfr_ptr))
	C.avio_context_free((**C.struct_AVIOContext)(unsafe.Pointer(avio_ctx)))
}

//export read_buffer_cb
func read_buffer_cb(opaque unsafe.Pointer, buf *C.uint8_t, buf_size C.int) C.int {
	ctx_ptr := (*Context)(opaque)
	avio_custom_buf := ContextBufferMap[ctx_ptr]
	data, data_len, ret := avio_custom_buf.ReadBuffer(int(buf_size))

	if ret < 0 {
		return C.int(ret)
	}

	if data_len >= 0 {
		C.memcpy(unsafe.Pointer(buf), unsafe.Pointer(&data[0]), C.size_t(data_len))
	}

	return C.int(data_len)
}

//export write_buffer_cb
func write_buffer_cb(opaque unsafe.Pointer, buf *C.uint8_t, buf_size C.int) C.int {
	ctx_ptr := (*Context)(opaque)
	avio_custom_buf := ContextBufferMap[ctx_ptr]
	avio_custom_buf.WriteBuffer(C.GoBytes(unsafe.Pointer(buf), buf_size))

	return 0
}
