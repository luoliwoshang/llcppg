// Package gowrite provides a stable output path for generated Go AST files
// whose nodes often have missing or partial token positions.
//
// Why this package exists:
//
// Generated AST from C/C++ conversion commonly contains token.NoPos on many
// declaration-related nodes. Older formatting behavior sometimes tolerated this,
// but newer go/printer behavior is stricter about relative positions. Without
// explicit anchors, comments and declarations may collapse onto one line, move
// to unexpected places, or print unstable forms (for example around empty
// interface types in function signatures).
//
// What gowrite does:
//
//  1. Build a synthetic token.File and assign only the minimal declaration
//     anchors needed for stable formatting.
//  2. Run go/format (format.Node) using that synthetic FileSet.
//
// This package does not try to fully reconstruct original source locations.
// It only supplies enough deterministic positions for correct, readable output.
//
// High-level algorithm:
//
//   - Assign positions in one forward sequence while walking declarations.
//     For comment groups, each comment's Slash gets a new position, and the
//     cursor advances by the comment's line span (not just by comment count).
//     This preserves spacing between multiline comments and the following nodes.
//
//   - Register a synthetic token.File after assignment, then install a monotonic
//     line table with SetLines. Assigned positions align with the final file base
//     via FileSet.Base() from the same FileSet instance.
//
// Why comment line span matters:
//
// Consider:
//
//	/*
//	ExecuteFoo comment
//	*/
//	//go:linkname CustomExecuteFoo2 C.ExecuteFoo2
//	func CustomExecuteFoo2()
//
// If comments are advanced by +1 each, the block comment and //go:linkname may
// end up on the same logical line. Advancing by real line span prevents this.
//
// Empty interface handling scope:
//
// We intentionally handle only function-related signatures:
// - ast.FuncDecl.Type
// - type declarations whose TypeSpec is ast.FuncType
//
// This is enough for cases like ...interface{} in variadic signatures, while
// avoiding broad, file-wide interface rewrites.
//
// Final line table:
//
// The synthetic token.File uses a simple monotonic line table via SetLines.
// This gives go/printer consistent line boundaries for all assigned anchors.
//
// Non-goals:
//
// - No semantic AST transformation.
// - No full-source positional normalization.
// - No attempt to preserve original file offsets from C/C++ headers.
package gowrite

import (
	"bytes"
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
	anchorDecls(fset, logicalName, file)
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

func anchorDecls(fset *token.FileSet, filename string, file *ast.File) {
	// Allocate synthetic positions from the current fileset base so all assigned
	// token.Pos values belong to the same future token.File.
	base := fset.Base()
	next := base
	newPos := func() token.Pos {
		pos := token.Pos(next)
		next++
		return pos
	}
	skipLines := func(n int) {
		next += n
	}

	file.Package = newPos()
	anchorComments(file.Doc, newPos, skipLines)

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			anchorComments(d.Doc, newPos, skipLines)
			if d.Type != nil {
				d.Type.Func = newPos()
				anchorEmptyIfaceInFunc(d.Type, newPos)
			}
			if d.Body != nil {
				d.Body.Lbrace = newPos()
				d.Body.Rbrace = newPos()
			}
		case *ast.GenDecl:
			anchorComments(d.Doc, newPos, skipLines)
			d.TokPos = newPos()
			anchorEmptyIfaceInTypeFuncs(d, newPos)
		}
	}

	// Register the backing token.File after we know the highest synthetic offset.
	size := next - base
	tf := fset.AddFile(filename, base, size)

	// Use a monotonic 1-offset-per-line table; line/column is only used for
	// stable printer spacing in this synthetic file.
	lines := make([]int, next-base)
	for i := range lines {
		lines[i] = i
	}
	if ok := tf.SetLines(lines); !ok {
		panic("internal/gowrite: failed to set synthetic line table")
	}
}

func anchorComments(
	group *ast.CommentGroup,
	newPos func() token.Pos,
	skipLines func(int),
) {
	if group == nil {
		return
	}
	for _, c := range group.List {
		if c == nil {
			continue
		}
		c.Slash = newPos()
		// Advance by real line span so a multiline block comment doesn't collapse
		// onto the following directive/declaration line.
		skipLines(lineSpan(c.Text) - 1)
	}
}

func anchorEmptyIfaceInFunc(ft *ast.FuncType, newPos func() token.Pos) {
	ast.Inspect(ft, func(node ast.Node) bool {
		it, ok := node.(*ast.InterfaceType)
		if !ok {
			return true
		}
		if !needsEmptyIface(it) {
			return true
		}

		p := newPos()
		it.Interface = p
		// Keep `interface{}` compact by pinning all three tokens to one anchor.
		it.Methods.Opening = p
		it.Methods.Closing = p
		return true
	})
}

func anchorEmptyIfaceInTypeFuncs(d *ast.GenDecl, newPos func() token.Pos) {
	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		ft, ok := ts.Type.(*ast.FuncType)
		if !ok {
			continue
		}
		// Restrict empty-interface anchoring to function-type declarations only.
		anchorEmptyIfaceInFunc(ft, newPos)
	}
}

func needsEmptyIface(it *ast.InterfaceType) bool {
	if it.Interface != token.NoPos {
		return false
	}
	if it.Methods == nil {
		return false
	}
	if len(it.Methods.List) != 0 {
		return false
	}
	return it.Methods.Opening == token.NoPos && it.Methods.Closing == token.NoPos
}

func lineSpan(text string) int {
	if text == "" {
		return 1
	}
	return strings.Count(text, "\n") + 1
}
