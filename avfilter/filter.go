// Use of this source code is governed by a MIT license that can be found in the LICENSE file.
// Giorgis (habtom@giorgis.io)

package avfilter

//#cgo pkg-config: libavfilter
//#include <libavfilter/avfilter.h>
import "C"
import "unsafe"

//Get a filter definition matching the given name.
func AvfilterGetByName(n string) *Filter {
	cn := C.CString(n)
	defer C.free(unsafe.Pointer(cn))
	return (*Filter)(C.avfilter_get_by_name(cn))
}
