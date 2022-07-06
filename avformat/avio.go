package avformat

//#cgo pkg-config: libavformat libavcodec libavutil libavdevice libavfilter libswresample libswscale
//#include <stdio.h>
//#include <stdlib.h>
//#include <libavformat/avformat.h>
//#include <libavformat/avio.h>
//extern int read_packet(void*, uint8_t*, int);
//extern int write_packet(void*, uint8_t*, int);
//extern int64_t seek(void*, int64_t, int);
import "C"
import (
	"bytes"
	"fmt"
	"sync"
	"unsafe"

	"github.com/skemtoputaete/goav/avutil"
)

const (
	SEEK_SET    = C.SEEK_SET
	SEEK_CUR    = C.SEEK_CUR
	SEEK_END    = C.SEEK_END
	AVSEEK_SIZE = C.AVSEEK_SIZE
)

type CustomIO interface {
	ReadBuffer(n_bytes int, buf unsafe.Pointer) (int, int)
	WriteBuffer(data []byte) int
}

type AvioCustomBuffer struct {
	CopyBuf []byte
	Buffer  *bytes.Buffer
	CustomIO
}

var ContextBufferMap sync.Map

func (custom_buf *AvioCustomBuffer) ReadBuffer(n_bytes int, buf unsafe.Pointer) (int, int) {
	bytes_to_read := n_bytes
	if n_bytes > custom_buf.Buffer.Len() {
		bytes_to_read = custom_buf.Buffer.Len()
	}

	if bytes_to_read == 0 {
		return 0, avutil.AVERROR_EOF
	}

	read_bytes, err := custom_buf.Buffer.Read(custom_buf.CopyBuf)
	if err != nil {
		return 0, avutil.AVERROR_EOF
	}

	C.memcpy(buf, unsafe.Pointer(&custom_buf.CopyBuf[0]), C.size_t(bytes_to_read))

	return read_bytes, 0
}

func (custom_buf *AvioCustomBuffer) WriteBuffer(data []byte) int {
	write_bytes, err := custom_buf.Buffer.Write(data)

	if err != nil {
		return 0
	}

	return write_bytes
}

func AvioAllocContext(fmt_ctx *Context, custom_buf CustomIO, buf *uint8, buf_size int, write_flag int, seekable bool) *AvIOContext {
	read_cb := (*[0]byte)(C.read_packet)
	write_cb := (*[0]byte)(C.write_packet)
	seek_cb := (*[0]byte)(C.seek)
	if write_flag == 0 {
		write_cb = nil
	} else {
		read_cb = nil
	}
	if !seekable {
		seek_cb = nil
	}

	avio_context := (*AvIOContext)(
		C.avio_alloc_context(
			(*C.uchar)(unsafe.Pointer(buf)), // Pointer to buffer
			C.int(buf_size),                 // Size of buffer
			C.int(write_flag),               // Buffer userd for writing
			unsafe.Pointer(fmt_ctx),         // Custom user specified data
			read_cb,                         // Function for reading packets
			write_cb,                        // Function for writing packets
			seek_cb,                         // Function for seeking
		),
	)
	ContextBufferMap.Store(fmt_ctx, custom_buf)

	return avio_context
}

func AvioFlush(avio_ctx *AvIOContext) {
	C.avio_flush((*C.struct_AVIOContext)(avio_ctx))
}

//Close the resource accessed by the AVIOContext *s, free it and set the pointer pointing to it to NULL.
func AvIOClosep(pb **AvIOContext) int {
	return int(C.avio_closep((**C.struct_AVIOContext)(unsafe.Pointer(pb))))
}

func AvioContextFree(avio_ctx **AvIOContext) {
	fmt_ctx := (*Context)((*avio_ctx).opaque)
	ContextBufferMap.Delete(fmt_ctx)

	bfr_ptr := (*avio_ctx).buffer
	if bfr_ptr != nil {
		avutil.AvFreep(unsafe.Pointer(&bfr_ptr))
	}
	C.avio_context_free((**C.struct_AVIOContext)(unsafe.Pointer(avio_ctx)))
}

func (avio_ctx *AvIOContext) Pos() int {
	return int(avio_ctx.pos)
}

func (avio_ctx *AvIOContext) BufferSize() int {
	return int(avio_ctx.buffer_size)
}

func (avio_ctx *AvIOContext) EofReached() int {
	return int(avio_ctx.eof_reached)
}

func (avio_ctx *AvIOContext) Dump() string {
	return fmt.Sprintf(
		"AVIOContext dump: \n\t position: %d \n\t buffer size: %d \n\t eof: %d \n\t buf ptr: %d \n\t buf end: %d \n",
		avio_ctx.Pos(),
		avio_ctx.BufferSize(),
		avio_ctx.EofReached(),
		avio_ctx.buf_ptr,
		avio_ctx.buf_end,
	)
}

func AvioFeof(avio_ctx *AvIOContext) int {
	return int(C.avio_feof((*C.struct_AVIOContext)(avio_ctx)))
}

//export read_packet
func read_packet(opaque unsafe.Pointer, buf *C.uint8_t, buf_size C.int) C.int {
	ctx_ptr := (*Context)(opaque)

	var avio_ctx CustomIO
	value, ok := ContextBufferMap.Load(ctx_ptr)
	if ok {
		avio_ctx = value.(CustomIO)
	}
	data_len, ret := avio_ctx.ReadBuffer(int(buf_size), unsafe.Pointer(buf))

	if ret < 0 {
		return C.int(ret)
	}

	return C.int(data_len)
}

//export write_packet
func write_packet(opaque unsafe.Pointer, buf *C.uint8_t, buf_size C.int) C.int {
	ctx_ptr := (*Context)(opaque)
	var avio_cb CustomIO
	value, ok := ContextBufferMap.Load(ctx_ptr)
	if ok {
		avio_cb = value.(CustomIO)
	}
	return C.int(avio_cb.WriteBuffer(C.GoBytes(unsafe.Pointer(buf), buf_size)))
}

//export seek
func seek(opaque unsafe.Pointer, pos C.int64_t, whence C.int) C.int64_t {
	ctx_ptr := (*Context)(opaque)
	var avio_cb CustomIO
	value, ok := ContextBufferMap.Load(ctx_ptr)
	if ok {
		avio_cb = value.(CustomIO)
	}
	if seek_avio_cb, ok := avio_cb.(interface{ Seek(int, int) int }); ok {
		return C.int64_t(seek_avio_cb.Seek(int(pos), int(whence)))
	}
	return avutil.AVERROR_EINVAL
}
