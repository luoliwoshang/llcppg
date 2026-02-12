package gowrite

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
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
	got := buf.String()
	want := "package demo\n\nfunc InitHooks() {\n}\n"
	if got != want {
		t.Fatalf("unexpected output.\nwant:\n%s\ngot:\n%s", want, got)
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
	got := buf.String()
	want := "package demo\n\nfunc RetZero() int {\n\treturn 0\n}\n"
	if got != want {
		t.Fatalf("unexpected output.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestAssignDeclAnchors_EmptyInterfaceStaysCompact(t *testing.T) {
	file := &ast.File{
		Name: ast.NewIdent("demo"),
		Decls: []ast.Decl{
			&ast.FuncDecl{
				Name: ast.NewIdent("Mprintf"),
				Type: &ast.FuncType{
					Params: &ast.FieldList{
						List: []*ast.Field{
							{
								Names: []*ast.Ident{ast.NewIdent("format")},
								Type:  ast.NewIdent("string"),
							},
							{
								Names: []*ast.Ident{ast.NewIdent("__llgo_va_list")},
								Type: &ast.Ellipsis{
									Elt: &ast.InterfaceType{Methods: &ast.FieldList{}},
								},
							},
						},
					},
				},
			},
		},
	}

	fset := token.NewFileSet()
	if err := assignDeclAnchors(fset, "demo.go", file); err != nil {
		t.Fatalf("assignDeclAnchors failed: %v", err)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		t.Fatalf("format.Node failed: %v", err)
	}

	got := buf.String()
	want := "package demo\n\nfunc Mprintf(format string, __llgo_va_list ...interface{})\n"
	if got != want {
		t.Fatalf("unexpected output.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestAssignDeclAnchors_InvalidCommentReported(t *testing.T) {
	file := &ast.File{
		Name: ast.NewIdent("demo"),
		Decls: []ast.Decl{
			&ast.FuncDecl{
				Doc: &ast.CommentGroup{
					List: []*ast.Comment{
						{Text: ""},
					},
				},
				Name: ast.NewIdent("Foo"),
				Type: &ast.FuncType{
					Params: &ast.FieldList{},
				},
			},
		},
	}

	fset := token.NewFileSet()
	err := assignDeclAnchors(fset, "demo.go", file)
	if err == nil {
		t.Fatal("expect invalid comment error, got nil")
	}
}

func TestAssignDeclAnchors_MultiLineDocAndLinkStaySeparated(t *testing.T) {
	file := &ast.File{
		Name: ast.NewIdent("demo"),
		Decls: []ast.Decl{
			&ast.FuncDecl{
				Doc: &ast.CommentGroup{
					List: []*ast.Comment{
						{Text: "/*\nExecuteFoo comment\n*/"},
						{Text: "//go:linkname CustomExecuteFoo2 C.ExecuteFoo2"},
					},
				},
				Name: ast.NewIdent("CustomExecuteFoo2"),
				Type: &ast.FuncType{
					Params: &ast.FieldList{},
				},
			},
		},
	}

	fset := token.NewFileSet()
	if err := assignDeclAnchors(fset, "demo.go", file); err != nil {
		t.Fatalf("assignDeclAnchors failed: %v", err)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		t.Fatalf("format.Node failed: %v", err)
	}

	got := buf.String()
	want := "package demo\n\n/*\nExecuteFoo comment\n*/\n//go:linkname CustomExecuteFoo2 C.ExecuteFoo2\nfunc CustomExecuteFoo2()\n"
	if got != want {
		t.Fatalf("unexpected output.\nwant:\n%s\ngot:\n%s", want, got)
	}
}
