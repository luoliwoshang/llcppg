// NOTE(zzy):temp define in current directory, need to be removed when support libclang at llpkg
package clang

import (
	_ "unsafe"

	"github.com/goplus/lib/c"
	"github.com/goplus/lib/c/clang"
)

const (
	LLGoFiles   = "$(llvm-config --cflags): _wrap/wrap.cpp"
	LLGoPackage = "link: -L$(llvm-config --libdir) -lclang; -lclang"
)

//go:linkname wrapIsCursorDefinition C.wrap_clang_isCursorDefinition
func wrapIsCursorDefinition(c *clang.Cursor) c.Int

func IsCursorDefinition(c clang.Cursor) c.Int {
	return wrapIsCursorDefinition(&c)
}
