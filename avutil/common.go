package avutil

//#cgo pkg-config: libavutil
//#include <libavutil/avutil.h>
//#include <libavutil/common.h>
//int bind_ffsign(int64_t v) {
//  return FFSIGN(v);
//}
import "C"

func AvClip(a, amin, amax int) int {
	return (int)(C.av_clip_c(C.int(a), C.int(amin), C.int(amax)))
}

func AvFfsign(v int64) int64 {
	return int64(C.bind_ffsign(C.int64_t(v)))
}
