package main

import (
	"gpgerror"

	"github.com/goplus/llgo/c"
)

var SOURCEMASK = gpgerror.SourceTSOURCEDIM - 1
var CODEMASK = gpgerror.CodeTCODEDIM - 1

func ErrMake(source gpgerror.SourceT, code gpgerror.CodeT) gpgerror.ErrorT {
	if code == gpgerror.CodeTNOERROR {
		return gpgerror.ErrorT(gpgerror.CodeTNOERROR)
	}

	return gpgerror.ErrorT(((c.Int(source) & c.Int(SOURCEMASK)) << gpgerror.SOURCESHIFT) | (c.Int(code) & c.Int(CODEMASK)))
}

func main() {
	gpgerror.Init()
	err := ErrMake(gpgerror.SourceTSOURCEUSER1, gpgerror.CodeTBADKEY)
	errStr := err.Strerror()
	source := err.Strsource()
	c.Printf(c.Str("Error: %s\n"), errStr)
	c.Printf(c.Str("Source: %s\n"), source)
}
