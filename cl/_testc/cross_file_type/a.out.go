package foo

import (
	"github.com/goplus/lib/c"
	"unsafe"
)

type Point struct {
	X    c.Int
	Data unsafe.Pointer
}
