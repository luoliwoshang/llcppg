package gowrite

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	if err := assignDeclAnchors(fset, logicalName, file); err != nil {
		return err
	}
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

func assignDeclAnchors(fset *token.FileSet, filename string, file *ast.File) error {
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
	total += countEmptyInterfaceAnchors(file)

	size := total + 8
	tf := fset.AddFile(filename, -1, size)
	next := 0
	newPos := func() token.Pos {
		pos := tf.Pos(next)
		next++
		return pos
	}
	skipLines := func(n int) {
		if n > 0 {
			next += n
		}
	}

	file.Package = newPos()
	if err := assignCommentGroup(file.Doc, newPos, skipLines, "file doc"); err != nil {
		return err
	}

	for i, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			name := "<anonymous>"
			if d.Name != nil && d.Name.Name != "" {
				name = d.Name.Name
			}
			if err := assignCommentGroup(d.Doc, newPos, skipLines, "func "+name); err != nil {
				return err
			}
			if d.Type != nil {
				d.Type.Func = newPos()
			}
			if d.Body != nil {
				d.Body.Lbrace = newPos()
				d.Body.Rbrace = newPos()
			}
		case *ast.GenDecl:
			if err := assignCommentGroup(d.Doc, newPos, skipLines, fmt.Sprintf("gen decl #%d", i)); err != nil {
				return err
			}
			d.TokPos = newPos()
		}
	}
	assignEmptyInterfaceAnchors(file, newPos)

	lines := make([]int, next+1)
	for i := range lines {
		lines[i] = i
	}
	if ok := tf.SetLines(lines); !ok {
		panic("internal/gowrite: failed to set synthetic line table")
	}
	return nil
}

func assignCommentGroup(
	group *ast.CommentGroup,
	newPos func() token.Pos,
	skipLines func(int),
	owner string,
) error {
	if group == nil {
		return nil
	}
	for i, c := range group.List {
		if c == nil {
			continue
		}
		if err := validateCommentText(c.Text); err != nil {
			return fmt.Errorf("%s comment[%d]: %w", owner, i, err)
		}
		c.Slash = newPos()
		skipLines(commentLineSpan(c.Text) - 1)
	}
	return nil
}

func countCommentGroup(group *ast.CommentGroup) int {
	if group == nil {
		return 0
	}
	n := 0
	for _, c := range group.List {
		if c == nil {
			continue
		}
		n += commentLineSpan(c.Text)
	}
	return n
}

func countEmptyInterfaceAnchors(file *ast.File) (n int) {
	ast.Inspect(file, func(node ast.Node) bool {
		it, ok := node.(*ast.InterfaceType)
		if !ok {
			return true
		}
		if needsEmptyInterfaceAnchor(it) {
			n++
		}
		return true
	})
	return
}

func assignEmptyInterfaceAnchors(file *ast.File, newPos func() token.Pos) {
	ast.Inspect(file, func(node ast.Node) bool {
		it, ok := node.(*ast.InterfaceType)
		if !ok {
			return true
		}
		if !needsEmptyInterfaceAnchor(it) {
			return true
		}

		p := newPos()
		it.Interface = p
		if it.Methods == nil {
			it.Methods = &ast.FieldList{}
		}
		// Keep `interface{}` compact by pinning all three tokens to one anchor.
		it.Methods.Opening = p
		it.Methods.Closing = p
		return true
	})
}

func needsEmptyInterfaceAnchor(it *ast.InterfaceType) bool {
	if it == nil {
		return false
	}
	if it.Interface != token.NoPos {
		return false
	}
	if it.Methods == nil {
		return true
	}
	if len(it.Methods.List) != 0 {
		return false
	}
	return it.Methods.Opening == token.NoPos && it.Methods.Closing == token.NoPos
}

func validateCommentText(text string) error {
	if len(text) < 2 {
		return fmt.Errorf("invalid comment text %q (len=%d)", text, len(text))
	}
	if text[0] != '/' || (text[1] != '/' && text[1] != '*') {
		return fmt.Errorf("invalid comment prefix %q", text)
	}
	return nil
}

func commentLineSpan(text string) int {
	if text == "" {
		return 1
	}
	return strings.Count(text, "\n") + 1
}
