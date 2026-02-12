package gowrite

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/goplus/gogen"
)

// WriteFile writes a gogen file using a synthetic-position pass before formatting.
// This makes declaration comments stable even when source nodes do not carry positions.
func WriteFile(pkg *gogen.Package, outFile string, fname ...string) error {
	var buf bytes.Buffer
	if gogen.GeneratedHeader != "" {
		buf.WriteString(gogen.GeneratedHeader)
	}
	if err := WriteTo(&buf, pkg, fname...); err != nil {
		return err
	}
	return os.WriteFile(outFile, buf.Bytes(), 0644)
}

// WriteTo formats a gogen file after injecting minimal declaration anchors.
func WriteTo(dst io.Writer, pkg *gogen.Package, fname ...string) error {
	file, logicalName, err := astFile(pkg, fname...)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
	assignDeclAnchors(fset, logicalName, file)
	return format.Node(dst, fset, file)
}

func astFile(pkg *gogen.Package, fname ...string) (*ast.File, string, error) {
	file := pkg.ASTFile(fname...)
	if file == nil {
		return nil, "", syscall.ENOENT
	}

	name := pkg.Types.Name() + ".go"
	if len(fname) > 0 && fname[0] != "" {
		name = fname[0]
	}
	name = filepath.Base(name)
	return file, name, nil
}

func assignDeclAnchors(fset *token.FileSet, filename string, file *ast.File) {
	total := 1 // package anchor
	total += countCommentGroup(file.Doc)
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			total++
			total += countCommentGroup(d.Doc)
			if d.Body != nil {
				total += 2 // { and } anchors
			}
		case *ast.GenDecl:
			total++
			total += countCommentGroup(d.Doc)
		}
	}

	size := total + 8
	tf := fset.AddFile(filename, -1, size)
	next := 0
	newPos := func() token.Pos {
		pos := tf.Pos(next)
		next++
		return pos
	}

	file.Package = newPos()
	assignCommentGroup(file.Doc, newPos)

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			assignCommentGroup(d.Doc, newPos)
			if d.Type != nil {
				d.Type.Func = newPos()
			}
			if d.Body != nil {
				d.Body.Lbrace = newPos()
				d.Body.Rbrace = newPos()
			}
		case *ast.GenDecl:
			assignCommentGroup(d.Doc, newPos)
			d.TokPos = newPos()
		}
	}

	lines := make([]int, next+1)
	for i := range lines {
		lines[i] = i
	}
	if ok := tf.SetLines(lines); !ok {
		panic("internal/gowrite: failed to set synthetic line table")
	}
}

func assignCommentGroup(group *ast.CommentGroup, newPos func() token.Pos) {
	if group == nil {
		return
	}
	for _, c := range group.List {
		c.Slash = newPos()
	}
}

func countCommentGroup(group *ast.CommentGroup) int {
	if group == nil {
		return 0
	}
	return len(group.List)
}
