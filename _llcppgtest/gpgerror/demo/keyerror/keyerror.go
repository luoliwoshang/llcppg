package main

import (
	"gpgerror"

	"github.com/goplus/lib/c"
)

var SOURCEMASK = gpgerror.SOURCE_DIM - 1
var CODEMASK = gpgerror.CODE_DIM - 1

func ErrMake(source gpgerror.SourceT, code gpgerror.CodeT) gpgerror.ErrorT {
	if code == gpgerror.NO_ERROR {
		return gpgerror.ErrorT(gpgerror.NO_ERROR)
	}

	return gpgerror.ErrorT(((c.Int(source) & c.Int(SOURCEMASK)) << gpgerror.SOURCE_SHIFT) | (c.Int(code) & c.Int(CODEMASK)))
}

func main() {
	gpgerror.Init()
	err := ErrMake(gpgerror.SOURCE_USER_1, gpgerror.BAD_KEY)
	errStr := err.Strerror()
	source := err.Strsource()
	c.Printf(c.Str("Error: %s\n"), errStr)
	c.Printf(c.Str("Source: %s\n"), source)
}
