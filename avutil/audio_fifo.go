package avutil

//#cgo pkg-config: libavutil
//#include <libavutil/audio_fifo.h>
//#include <stdlib.h>
import "C"
import "unsafe"

func (a *AvAudioFifo) AvAudioFifoRealloc(nbSamples int) int {
	return int(C.av_audio_fifo_realloc((*C.struct_AVAudioFifo)(a), C.int(nbSamples)))
}

func (a *AvAudioFifo) AvAudioFifoSize() int {
	return int(C.av_audio_fifo_size((*C.struct_AVAudioFifo)(a)))
}

func (a *AvAudioFifo) AvAudioFifoWrite(data **uint8, nbSamples int) int {
	return int(C.av_audio_fifo_write((*C.struct_AVAudioFifo)(a), (*unsafe.Pointer)(unsafe.Pointer(data)), C.int(nbSamples)))
}

func (af *AvAudioFifo) AvAudioFifoRead(data **uint8, nbSamples int) int {
	return int(C.av_audio_fifo_read((*C.struct_AVAudioFifo)(af), (*unsafe.Pointer)(unsafe.Pointer(data)), C.int(nbSamples)))
}

func (af *AvAudioFifo) AvAudioFifoFree() {
	C.av_audio_fifo_free((*C.struct_AVAudioFifo)(af))
}
