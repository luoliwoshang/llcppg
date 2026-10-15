package foo

import (
	"github.com/goplus/lib/c"
	_ "unsafe"
)

const (
	LLGoPackage = "link: -L/path/foo -lfoo"
	LLGoFiles   = "-I/path/foo/include: _wrap/llcppg.c"
)

type GzFileS struct {
	Have c.Uint
	Next *uint8
	Pos  c.Long
}
type ZStreamS struct {
}
type GzFile = *GzFileS
type ZStream = ZStreamS
type ZStreamp = *ZStream

// Read one byte, falling back to the real function.
//
//go:linkname ZGzgetc C._llcppg_z_gzgetc
func ZGzgetc(g GzFile) c.Int

//go:linkname ZDeflateInit C._llcppg_z_deflateInit
func ZDeflateInit(strm ZStreamp, level c.Int) c.Int

//go:linkname ZReset C._llcppg_z_reset
func ZReset(strm ZStreamp, flags c.Int)

// Read one byte, falling back to the real function.
//
// llgo:link GzFile.Gzgetc_ C.gzgetc_
func (file GzFile) Gzgetc_() c.Int {
	return 0
}

// llgo:link ZStreamp.DeflateInit_ C.deflateInit_
func (strm ZStreamp) DeflateInit_(level c.Int, version *c.Char, stream_size c.Int) c.Int {
	return 0
}

// llgo:link ZStreamp.Reset_ C.reset_
func (strm ZStreamp) Reset_(flags c.Int, version *c.Char) {
}
