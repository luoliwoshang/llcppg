package foo

import "github.com/goplus/lib/c"

type Foo struct {
	U      _llcppg_anon_0
	Shorts FooShorts
}
type Bar struct {
	_llcppg_anon_1
}
type Baz struct {
	A c.Int
	B c.Uint
	c c.Int
}
type FooShorts struct {
	S  int16
	Us uint16
}
type _llcppg_anon_0 struct {
	X c.Float
	Y c.Float
}
type _llcppg_anon_1 struct {
	S  int16
	Us uint16
}
