package types

import (
	"github.com/goplus/lib/c"
	"unsafe"
)

const XGoPackage = true

type Status c.Uint

const B_OK Status = 0

type Size = c.Ulong
type Count = c.Int
type Macro struct {
	X c.Int
}
type MacroAnon struct {
	X c.Int
}
type Point struct {
	X c.Int
	Y c.Int
}
type Alias = Point
type Anon struct {
	X c.Int
}
type Value struct {
	_xgo_union [1]uint32
}
type Handle struct {
}
type HandlePtr = *Handle
type Container struct {
	Inner _llcppg_anon_0
}
type _llcppg_anon_0 struct {
	X c.Int
}

// llgo:type C
type Callback = func(_llcppg_param1 c.Int)
type Configured = c.Long
type private = c.Int

func (p *Value) XGof_ref_n() *c.Int {
	return (*c.Int)(unsafe.Pointer(p))
}
func (p *Value) XGof_ref_f() *c.Float {
	return (*c.Float)(unsafe.Pointer(p))
}
