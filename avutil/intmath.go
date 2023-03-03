package avutil

//#cgo pkg-config: libavutil libavcodec
//#include <libavutil/avutil.h>
//int bind_av_log2(unsigned int v) {
//	return av_log2(v);
//}
import "C"

func AvLog2(v int) int {
	return int(C.bind_av_log2(C.uint(v)))
}
