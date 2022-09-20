package avformat

//#cgo pkg-config: libavformat
//#include <libavformat/avformat.h>
import "C"

func (inputFormat *InputFormat) Flags() int {
	return int(inputFormat.flags)
}

func (inputFormat *InputFormat) SetFlags(flags int) {
	inputFormat.flags = C.int(flags)
}
