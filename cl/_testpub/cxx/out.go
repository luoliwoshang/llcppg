package cxx

import (
	"github.com/goplus/lib/c"
	"unsafe"
)

const XGoPackage = true

type Color c.Int

const (
	Color_red   Color = 0
	Color_green Color = 1
)

type Widget struct {
	x c.Int
}
type Record struct {
	X c.Int
}
type Value struct {
	_xgo_union [1]uint32
}
type Index = c.Int
type Box[T any] struct {
	Value T
}

func (p *Value) XGof_ref_x() *c.Int {
	return (*c.Int)(unsafe.Pointer(p))
}
func (p *Value) XGof_ref_y() *c.Float {
	return (*c.Float)(unsafe.Pointer(p))
}
