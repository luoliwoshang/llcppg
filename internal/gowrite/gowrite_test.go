package gowrite

import (
	"bytes"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/goplus/gogen"
)

func TestWriteTo_EmptyFuncHasCompactBody(t *testing.T) {
	pkg := gogen.NewPackage("", "demo", nil)
	pkg.NewFunc(nil, "InitHooks", nil, nil, false).BodyStart(pkg).End()

	var buf bytes.Buffer
	if err := WriteTo(&buf, pkg, ""); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	code := buf.String()
	if !strings.Contains(code, "func InitHooks() {\n}") {
		t.Fatalf("empty func body format changed:\n%s", code)
	}
}

func TestWriteTo_NonEmptyFuncStaysMultiline(t *testing.T) {
	pkg := gogen.NewPackage("", "demo", nil)
	results := types.NewTuple(pkg.NewParam(token.NoPos, "", types.Typ[types.Int]))
	pkg.NewFunc(nil, "RetZero", nil, results, false).BodyStart(pkg).Val(0).Return(1).End()

	var buf bytes.Buffer
	if err := WriteTo(&buf, pkg, ""); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	code := buf.String()
	if strings.Contains(code, "func RetZero() int { return 0 }") {
		t.Fatalf("non-empty func collapsed to one-line:\n%s", code)
	}
}
