package convert_test

import (
	"testing"

	"github.com/goplus/llcppg/ast"
	"github.com/goplus/llcppg/cmd/gogensig/convert"
)

func TestTypes(t *testing.T) {
	types := convert.NewTypes()
	types.Register("usr1", &ast.TypeDecl{Name: &ast.Ident{Name: "usr1"}})
	types.Register("usr2", &ast.TypeDecl{Name: &ast.Ident{Name: "usr2"}})
	node, ok := types.Lookup("usr1")
	if !ok {
		t.Fatal("Expect true")
	}
	decl, ok := node.(*ast.TypeDecl)
	if !ok {
		t.Fatal("Expect *ast.TypeDecl")
	}
	if decl.Name.Name != "usr1" {
		t.Fatal("Expect usr1")
	}
}
