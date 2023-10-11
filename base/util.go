package base

import (
	"errors"
	"reflect"
	"unsafe"
)

type Util struct {
}

//check channel closed or not
func (f *Util) CheckChanClosed(ch interface{}) (bool, error) {
	//check
	if reflect.TypeOf(ch).Kind() != reflect.Chan {
		return true, errors.New("input value must be channel kind")
	}

	// get interface value pointer, from cgo_export
	// typedef struct { void *t; void *v; } GoInterface;
	// then get channel real pointer
	ptr := *(*uintptr)(unsafe.Pointer(
		unsafe.Pointer(uintptr(unsafe.Pointer(&ch)) + unsafe.Sizeof(uint(0))),
	))

	// this function will return true if chan.closed > 0
	// see hchan on https://github.com/golang/go/blob/master/src/runtime/chan.go
	// type hchan struct {
	// qcount   uint           // total data in the queue
	// dataqsiz uint           // size of the circular queue
	// buf      unsafe.Pointer // points to an array of dataqsiz elements
	// elemsize uint16
	// closed   uint32
	// **

	ptr += unsafe.Sizeof(uint(0))*2
	ptr += unsafe.Sizeof(unsafe.Pointer(uintptr(0)))
	ptr += unsafe.Sizeof(uint16(0))
	return *(*uint32)(unsafe.Pointer(ptr)) > 0, nil
}