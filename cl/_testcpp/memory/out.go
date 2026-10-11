package foo

import "github.com/goplus/lib/c"

type SizeT = c.Ulong
type WeakPtr[_Tp any] struct {
	__Ptr_   *_Tp
	__Cntrl_ *SizeT
}
type X__SharedWeakCount struct {
}
type X__SharedPtrEmplace_Storage_Data struct {
	X__Alloc_ _Alloc
	X__Elem_  _Tp
}
type SharedPtr[_Tp any] struct {
	__Ptr_   *_Tp
	__Cntrl_ *SizeT
}
type SharedPtrWeakType[_Tp any] = WeakPtr[_Tp]
