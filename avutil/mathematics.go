package avutil

//#include <libavutil/mathematics.h>
import "C"

const (
	AV_ROUND_ZERO        = C.AV_ROUND_ZERO
	AV_ROUND_INF         = C.AV_ROUND_INF
	AV_ROUND_DOWN        = C.AV_ROUND_DOWN
	AV_ROUND_UP          = C.AV_ROUND_UP
	AV_ROUND_NEAR_INF    = C.AV_ROUND_NEAR_INF
	AV_ROUND_PASS_MINMAX = C.AV_ROUND_PASS_MINMAX
)

func AvRescaleRnd(a int64, b int64, c int64, flags uint32) int64 {
	return (int64)(C.av_rescale_rnd(C.int64_t(a), C.int64_t(b), C.int64_t(c), flags))
}

func AvRescaleQ(a int64, bq Rational, cq Rational) int64 {
	return (int64)(C.av_rescale_q(C.int64_t(a), (C.struct_AVRational)(bq), (C.struct_AVRational)(cq)))
}

func AvRescaleQRnd(a int64, bq Rational, cq Rational, flags uint32) int64 {
	return (int64)(C.av_rescale_q_rnd(C.int64_t(a), (C.struct_AVRational)(bq), (C.struct_AVRational)(cq), flags))
}
