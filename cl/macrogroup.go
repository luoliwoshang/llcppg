/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cl

import (
	"github.com/goplus/gogen"
	"github.com/goplus/lib/c"
	"github.com/goplus/llcppg/clang"
	lc "github.com/llarhub/clang-c"
)

// -----------------------------------------------------------------------------
//
// MacroGroup support (see https://github.com/goplus/llcppg/issues/1064).
//
// A MacroGroup is an ordered run of two or more object-like macros emitted
// together as a single Go `const ( ... )` block instead of a flat list of
// single `const` declarations. Groups come in two kinds:
//
//   - Automatic: a run of consecutive macro definitions (no line gap between
//     them) whose values are all constant and all of the same untyped kind.
//   - Configured: declared through the MacroGroup config map (first macro name
//     => last macro name); every macro definition between the endpoints in
//     source order belongs to one group regardless of blank lines or comments.
//
// Grouping rides the existing preload walk in loadFiles — there is no separate
// traversal of the macros. groupVisit is called for every node of that walk, in
// source order, and drives a small state machine:
//
//   - Automatic runs are emitted as they close. A run ends at the first macro
//     that is not source-adjacent (or of a different value kind) or at the first
//     non-macro node; closing emits the run's `const ( ... )` block (or a plain
//     const for a lone macro) right then. Because libclang visits every macro
//     before any regular declaration, this happens while only macros have been
//     emitted, so macro consts keep their place ahead of the deferred type,
//     function and variable declarations.
//   - Configured groups (MacroGroup config: first macro name => last macro name)
//     are recorded as their boundaries are seen and emitted after the walk, by
//     groupFinish, once their ranges have been validated against the non-macro
//     declarations libclang only reveals after all macros.
//
// Either way emitMacroGroup evaluates the members, applies the constant/same-type
// rule for automatic groups, and emits the `const ( ... )` block(s).

// current (in-progress) group state.
type macroGrouper struct {
	file     string // file of this group
	lastLine int    // last line of the previous member (covers backslash continuation)
	first    string // name of the first macro in the current group
	last     string // expected last macro name; non-empty => configured group
	items    []clang.Cursor
	defs     *gogen.ConstDefs
}

// groupVisit feeds one visited node of the preload walk into the grouping state
// machine. libclang visits every macro definition (in source order) before any
// regular declaration, so non-macro nodes are not interleaved with macros during
// the walk. The state machine steps on object-like macros, emitting each
// automatic run as it closes; the first non-macro node closes (and emits) any
// open automatic run, and is also recorded by source line so groupFinish can
// validate that configured ranges contain only macro definitions.
func groupVisit(ctx *pkgCtx, decl clang.Cursor) bool {
	g := ctx.mgroup

	if g != nil && g.last != "" {
		if decl.Kind != lc.Cursor_MacroDefinition {
			ctx.errorf(decl, "macroGroup %q..%q: expected last macro not found", g.first, g.last)
			groupClose(ctx)
			return false
		}
		if decl.IsMacroFunctionLike() != 0 {
			return true // ignore function-like macros
		}
		g.items = append(g.items, decl)
		if clang.String(decl) == g.last {
			groupClose(ctx)
		}
		return true
	}

	if decl.Kind != lc.Cursor_MacroDefinition {
		return false
	}

	if decl.IsMacroFunctionLike() != 0 {
		preloadMacroFunc(ctx, decl)
		return true
	}

	name := clang.String(decl)
	if last, ok := ctx.macroGroup[name]; ok {
		groupClose(ctx)
		groupStartConfigured(ctx, decl, name, last)
		return true
	}

	file, firstLine, lastLine := macroExtentLines(decl)
	if g != nil && g.file == file && g.lastLine+1 == firstLine {
		g.items = append(g.items, decl)
		g.lastLine = lastLine
	} else {
		groupClose(ctx)
		groupStartAuto(ctx, decl, file, lastLine)
	}
	return true
}

func groupNew(ctx *pkgCtx, decl clang.Cursor) (ret *macroGrouper) {
	pkg := ctx.pkg
	ret = &macroGrouper{
		defs:  pkg.NewConstDefs(pkg.Types.Scope()),
		items: []clang.Cursor{decl},
	}
	ctx.mgroup = ret
	return
}

func groupStartAuto(ctx *pkgCtx, decl clang.Cursor, file string, lastLine int) {
	g := groupNew(ctx, decl)
	g.file = file
	g.lastLine = lastLine
}

func groupStartConfigured(ctx *pkgCtx, decl clang.Cursor, first, last string) {
	g := groupNew(ctx, decl)
	g.first = first
	g.last = last
}

func groupClose(ctx *pkgCtx) {
	if g := ctx.mgroup; g != nil {
		ctx.mgroup = nil
		if len(g.items) == 1 {
			loadMacroConst(ctx, g.defs, g.items[0])
		} else {
			loadMacroConstGroup(ctx, g.defs, g.items)
		}
	}
}

// macroExtentLines returns the file name and the first and last expansion lines
// of a macro definition's extent. The last line already accounts for backslash
// continuation.
func macroExtentLines(decl clang.Cursor) (file string, firstLine, lastLine int) {
	var f clang.File
	var line c.Uint
	rg := decl.Extent()
	rg.Start().Expansion(&f, &line, nil, nil)
	if f != clang.InvalidFile {
		file = clang.FileName(f)
	}
	firstLine = int(line)
	rg.End().Expansion(nil, &line, nil, nil)
	lastLine = int(line)
	return
}

// -----------------------------------------------------------------------------
