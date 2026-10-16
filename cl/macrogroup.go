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
	"go/ast"
	"log"

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
// The work is split across two stages, mirroring the rest of the pipeline:
//
//   - preload (macroGrouper): a single pass over the node stream records where
//     each group starts and ends, and the ordered list of member macros. It
//     decides boundaries only; it does not evaluate macro values.
//   - load (emitMacroGroup): when loadMacroConst reaches the first macro of a
//     recorded group, it evaluates the members, applies the constant/same-type
//     rule for automatic groups, and emits the `const ( ... )` block(s).

// macroGroup is a recorded group handed from preload to load, keyed by its
// first macro's original C name.
type macroGroup struct {
	members    []clang.Cursor // member macro cursors in source order, first..last inclusive
	configured bool           // true when the group came from the MacroGroup config
	file       string         // header file of the group (all members share it)
	firstLine  int            // first line of the first member
	lastLine   int            // last line of the last member
	emitted    bool           // guards against re-emission when a macro name is redefined
}

// macroGrouper is the preload-stage state machine that records macro groups by
// walking the node stream in source order.
type macroGrouper struct {
	records map[string]*macroGroup // first macro name => recorded group
	member  map[string]*macroGroup // every member macro name => its recorded group

	// current (in-progress) group state. hasCur reports whether a group is open.
	first     clang.Cursor   // first macro of the current group (name + cursor)
	firstFile string         // file of the first member
	firstLine int            // first line of the first member
	prevFile  string         // file of the previous member
	prevLine  int            // last line of the previous member (covers backslash continuation)
	expected  string         // expected last macro name; non-empty => configured group
	cur       []clang.Cursor // members accumulated for the current group

	// nonMacro maps a header file to the first lines of non-macro declarations in
	// it, in visit order (not sorted). libclang visits all macro definitions
	// before any regular declaration, so a non-macro node is never interleaved
	// with macros during the walk; instead its position is used to validate
	// configured ranges (only macro definitions may lie inside a range).
	// Populated only when there are configured groups to validate.
	nonMacro map[string][]int

	matched map[string]bool // MacroGroup keys that have been matched (first macro seen)
	seen    map[string]bool // macro name|file already processed (header seen from many sources)
	hasCur  bool            // whether there is a current group in progress
}

// preloadMacroGroups walks the node stream once, before the emission pass,
// recording macro group boundaries. It mirrors loadFiles' per-file visit and
// package/header filtering so the recorded boundaries match exactly the nodes
// the emission pass will process, but it does not mutate ctx.lastSeen.
func preloadMacroGroups(ctx *pkgCtx, files []Source, myPkgPath string) {
	if !ctx.macroGroupEnabled() {
		return
	}
	pkgOf := ctx.pkgOf
	// A local copy of lastSeen is advanced as files are walked, matching how the
	// emission pass carries forward already-loaded headers across files.
	lastSeen := make(map[string]none, len(ctx.lastSeen))
	for k, v := range ctx.lastSeen {
		lastSeen[k] = v
	}
	for _, f := range files {
		thisSeen := make(map[string]none)
		clang.VisitChildren(f.Cursor(), func(decl, parent clang.Cursor) clang.ChildVisitResult {
			if pkgOf != nil {
				at := clang.PresumedFile(decl.Location())
				if _, ok := lastSeen[at]; ok {
					return clang.Continue // already loaded
				}
				pkgPath, ok := pkgOf(at)
				if !ok || pkgPath != myPkgPath {
					return clang.Continue
				}
				thisSeen[at] = none{}
			}
			ctx.groupVisit(decl)
			return clang.Continue
		})
		for k, v := range thisSeen {
			lastSeen[k] = v
		}
	}
	ctx.groupFinish()
}

func newMacroGrouper() *macroGrouper {
	return &macroGrouper{
		records:  make(map[string]*macroGroup),
		member:   make(map[string]*macroGroup),
		nonMacro: make(map[string][]int),
		matched:  make(map[string]bool),
		seen:     make(map[string]bool),
	}
}

// macroGroupEnabled reports whether the macro-grouping preload pass should run.
// It is currently always on: automatic detection of adjacent constant-macro
// runs needs the pass regardless of whether a MacroGroup config is set, and
// grouping is a no-op layout change when there is nothing to group, so it is
// always safe to run. Configured groups are not gated here; they are driven
// purely by the contents of p.macroGroup (empty => automatic detection only).
// This method exists as the single seam through which grouping could be made
// opt-out in the future.
func (p *pkgCtx) macroGroupEnabled() bool {
	return true
}

// hasConfiguredGroups reports whether any configured MacroGroup range exists.
// The non-macro line bookkeeping used to validate configured ranges is only
// needed when this is true.
func (p *pkgCtx) hasConfiguredGroups() bool {
	return len(p.macroGroup) > 0
}

func (p *pkgCtx) grouper() *macroGrouper {
	if p.mgroup == nil {
		p.mgroup = newMacroGrouper()
	}
	return p.mgroup
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

// groupVisit feeds one visited node into the preload grouping state machine.
// libclang visits every macro definition (in source order) before any regular
// declaration, so non-macro nodes are not interleaved with macros during the
// walk. The state machine therefore only steps on object-like macros; non-macro
// declarations are recorded by source line and used afterward (groupFinish) to
// validate that configured ranges contain only macro definitions.
func (p *pkgCtx) groupVisit(decl clang.Cursor) {
	if !p.macroGroupEnabled() {
		return
	}
	g := p.grouper()

	isMacroDef := decl.Kind == lc.Cursor_MacroDefinition
	isObjMacro := isMacroDef && decl.IsMacroFunctionLike() == 0

	if !isObjMacro {
		// A non-macro declaration (or a function-like macro) is not a group
		// member. Record its line so a configured range that straddles it can be
		// rejected. Function-like macros are recorded too, since they may not sit
		// inside a configured range of object-like constant macros. This is only
		// needed to validate configured ranges, so it is skipped (along with its
		// per-node clang extent lookup) when there are no configured groups.
		if p.hasConfiguredGroups() {
			file, firstLine, _ := macroExtentLines(decl)
			if file != "" {
				g.nonMacro[file] = append(g.nonMacro[file], firstLine)
			}
		}
		return
	}

	name := clang.String(decl)
	file, firstLine, lastLine := macroExtentLines(decl)

	// A header reached from several sources contributes its macros once.
	key := name + "|" + file
	if g.seen[key] {
		return
	}
	g.seen[key] = true

	if g.hasCur && g.expected != "" {
		// Case 1: inside a configured group. Adjacency is not checked here.
		if file != g.prevFile {
			p.errorf(decl, "MacroGroup %q..%q: range crosses header files",
				clang.String(g.first), g.expected)
			g.reset()
			return
		}
		if _, isKey := p.macroGroup[name]; isKey && name != g.expected {
			p.errorf(decl, "MacroGroup %q..%q: overlaps another group starting at %q",
				clang.String(g.first), g.expected, name)
			g.reset()
			return
		}
		g.appendMember(decl, file, lastLine)
		if name == g.expected {
			g.record(p)
		}
		return
	}

	// Case 2: automatic detection (no expected last macro).
	if last, isKey := p.macroGroup[name]; isKey {
		if g.hasCur {
			g.closeAuto(p)
		}
		if last == name {
			// A range whose first and last macro are identical describes a single
			// macro, which is not a group. Report it directly instead of letting it
			// fall through to the misleading "last macro not found" error.
			p.errorf(decl, "MacroGroup %q..%q: first and last macro are the same (a group needs at least two macros)",
				name, last)
			g.matched[name] = true
			return
		}
		g.start(decl, file, firstLine, lastLine, last)
		g.matched[name] = true
		return
	}
	if !g.hasCur {
		g.start(decl, file, firstLine, lastLine, "")
		return
	}
	if file != g.prevFile || firstLine != g.prevLine+1 {
		g.closeAuto(p)
		g.start(decl, file, firstLine, lastLine, "")
		return
	}
	g.appendMember(decl, file, lastLine)
}

// start opens a new current group at decl. expected is the awaited last macro
// name for a configured group, or "" for an automatic group; it is the single
// place g.expected is assigned when a group opens.
func (g *macroGrouper) start(decl clang.Cursor, file string, firstLine, lastLine int, expected string) {
	g.first = decl
	g.firstFile = file
	g.firstLine = firstLine
	g.prevFile = file
	g.prevLine = lastLine
	g.cur = []clang.Cursor{decl}
	g.hasCur = true
	g.expected = expected
}

func (g *macroGrouper) appendMember(decl clang.Cursor, file string, lastLine int) {
	g.prevFile = file
	g.prevLine = lastLine
	g.cur = append(g.cur, decl)
}

func (g *macroGrouper) reset() {
	g.firstFile = ""
	g.firstLine = 0
	g.prevFile = ""
	g.prevLine = 0
	g.expected = ""
	g.cur = nil
	g.hasCur = false
}

// record finalizes the current group (configured, or any group with >= 2
// members) and stores it keyed by the first macro name.
func (g *macroGrouper) record(p *pkgCtx) {
	if g.hasCur && len(g.cur) >= 2 {
		rec := &macroGroup{
			members:    g.cur,
			configured: g.expected != "",
			file:       g.firstFile,
			firstLine:  g.firstLine,
			lastLine:   g.prevLine,
		}
		firstName := clang.String(g.first)
		g.records[firstName] = rec
		for _, m := range g.cur {
			g.member[clang.String(m)] = rec
		}
	}
	g.reset()
}

// closeAuto closes an in-progress automatic group. It defers to record, which
// keeps groups of two or more members and discards a single-macro run (not a
// group). The separate name marks, at each call site, that an automatic run is
// ending (vs. a configured range closing explicitly on its last macro).
func (g *macroGrouper) closeAuto(p *pkgCtx) {
	g.record(p)
}

// groupFinish flushes any in-progress group at the end of the input and reports
// the remaining configuration errors: an unterminated configured range (the
// last macro was never found after the first), any MacroGroup key that never
// matched a macro (checkUnmatchedKeys), and any configured range that straddles
// a non-macro declaration (validateConfiguredRanges).
func (p *pkgCtx) groupFinish() {
	if !p.macroGroupEnabled() || p.mgroup == nil {
		p.checkUnmatchedKeys()
		return
	}
	g := p.mgroup
	if g.hasCur {
		if g.expected != "" {
			// The expected last macro was never found: either an unknown last
			// macro or a reversed range (last before first).
			p.errorf(g.first, "MacroGroup %q..%q: last macro not found after first (unknown or reversed range)",
				clang.String(g.first), g.expected)
			g.reset()
		} else {
			g.closeAuto(p)
		}
	}
	p.checkUnmatchedKeys()
	g.validateConfiguredRanges(p)
}

// validateConfiguredRanges rejects any configured group that straddles a
// non-macro declaration. libclang does not interleave non-macro nodes with
// macros during the walk, so this positional check (rather than visit order)
// enforces "only macro definitions may appear inside a range".
func (g *macroGrouper) validateConfiguredRanges(p *pkgCtx) {
	for first, rec := range g.records {
		if !rec.configured {
			continue
		}
		for _, line := range g.nonMacro[rec.file] {
			if line > rec.firstLine && line < rec.lastLine {
				// Reported through errorf (like the other configured-range errors)
				// so the message carries the first macro's source position and the
				// errCnt/fail-fast bookkeeping stays in one place. The offending
				// line is still named explicitly since it differs from the group.
				p.errorf(rec.members[0], "MacroGroup %q..%q: non-macro declaration inside range (line %d)",
					first, clang.String(rec.members[len(rec.members)-1]), line)
				delete(g.records, first)
				for _, m := range rec.members {
					delete(g.member, clang.String(m))
				}
				break
			}
		}
	}
}

// checkUnmatchedKeys reports every MacroGroup first-macro key that never matched
// a macro definition. These keys have no corresponding cursor (the macro was
// never seen), so the error cannot carry a source position. It reads the matched
// set from p.mgroup directly, so it is correct even when no grouper was ever
// created (p.mgroup == nil => nothing matched).
func (p *pkgCtx) checkUnmatchedKeys() {
	var matched map[string]bool
	if p.mgroup != nil {
		matched = p.mgroup.matched
	}
	for first := range p.macroGroup {
		if !matched[first] {
			p.errorNoPos("MacroGroup: unknown first macro %q", first)
		}
	}
}

// errorNoPos reports a MacroGroup configuration error that has no source
// position (the offending macro was never seen). It mirrors errorf's errCnt and
// fail-fast bookkeeping so every MacroGroup error is counted the same way; it
// just cannot prefix a file:line position.
func (p *pkgCtx) errorNoPos(format string, args ...any) {
	p.errCnt++
	log.Printf(format, args...)
	p.checkFailFast()
}

// -----------------------------------------------------------------------------

// macroGroupOf returns the group whose first macro is name, or nil.
func (p *pkgCtx) macroGroupOf(name string) *macroGroup {
	if p.mgroup == nil {
		return nil
	}
	return p.mgroup.records[name]
}

// isGroupMember reports whether name is a member (first or otherwise) of any
// recorded group. Members other than the first are emitted by the group's first
// macro, so their own loadMacroConst dispatch is a no-op.
func (p *pkgCtx) isGroupMember(name string) bool {
	if p.mgroup == nil {
		return false
	}
	_, ok := p.mgroup.member[name]
	return ok
}

// -----------------------------------------------------------------------------
//
// Load stage: emit the const block(s) for a group.

// groupMember is a resolved member of a group being emitted.
type groupMember struct {
	decl     clang.Cursor
	origName string // original C macro name
	name     string // Go const name
	val      any    // evaluated constant value
	kind     int    // type tag: 0 int, 1 float, 2 string (only meaningful for auto split)
}

func constKind(v any) int {
	switch v.(type) {
	case int:
		return 0
	case float64:
		return 1
	case string:
		return 2
	default:
		return -1
	}
}

// emitMacroGroup emits the Go declarations for a recorded group whose first
// macro is first. It is called from loadMacroConst when it reaches the first
// macro of a group.
func emitMacroGroup(ctx *pkgCtx, grp *macroGroup) {
	// Resolve the group's doc comment from the first macro's preceding comment
	// (priority) or the last macro's trailing comment.
	firstRaw := clang.RawCommentsOf(grp.members[0])
	lastRaw := clang.RawCommentsOf(grp.members[len(grp.members)-1])
	var precDoc, trailDoc *ast.CommentGroup
	if ctx.keepDoc {
		if firstRaw.Preceding != "" {
			precDoc = docCommentGroupRaw(toLineComments(firstRaw.Preceding))
		} else if lastRaw.Trailing != "" {
			trailDoc = docCommentGroupRaw(toLineComments(lastRaw.Trailing))
		}
	}

	// Evaluate every member. Non-constant or function-like members are dropped
	// from the group and handled by the normal pipeline, with a warning.
	resolved := make([]groupMember, 0, len(grp.members))
	for _, m := range grp.members {
		name := clang.String(m)
		if ctx.isMacroIgnored(name) {
			continue // dropped from the group by MacroIgnore, like a standalone macro
		}
		v, ok := evalMacro(ctx, m)
		if !ok {
			log.Printf("[WARN] MacroGroup: non-constant macro %q skipped from group", name)
			continue
		}
		// Record the value now so a later member that aliases this one (e.g.
		// Z_ASCII => Z_TEXT) resolves while the group is still being evaluated.
		ctx.macroVals[name] = v
		resolved = append(resolved, groupMember{decl: m, origName: name, name: ctx.macroName(name), val: v, kind: constKind(v)})
	}
	if len(resolved) == 0 {
		return
	}

	firstName := clang.String(grp.members[0])
	lastName := clang.String(grp.members[len(grp.members)-1])

	if grp.configured {
		// A configured group is a single block; mixed untyped kinds are allowed.
		emitConstBlock(ctx, resolved, precDoc, trailDoc, firstName, lastName)
		return
	}

	// Automatic group: split into sub-runs by constant kind (Rule 2). The
	// preceding comment goes to the first block; the trailing comment goes to the
	// block containing the last macro.
	start := 0
	for start < len(resolved) {
		end := start + 1
		for end < len(resolved) && resolved[end].kind == resolved[start].kind {
			end++
		}
		sub := resolved[start:end]
		var subPrec, subTrail *ast.CommentGroup
		if start == 0 {
			subPrec = precDoc
		}
		if sub[len(sub)-1].origName == lastName {
			subTrail = trailDoc
		}
		emitConstBlock(ctx, sub, subPrec, subTrail, firstName, lastName)
		start = end
	}
}

// emitConstBlock emits one Go const declaration for the given members. A single
// member degrades to a plain `const`; two or more are emitted as a
// `const ( ... )` block. precDoc (if any) becomes the block/const doc; trailDoc
// is used only when precDoc is nil.
func emitConstBlock(ctx *pkgCtx, members []groupMember, precDoc, trailDoc *ast.CommentGroup, firstName, lastName string) {
	doc := precDoc
	if doc == nil {
		doc = trailDoc
	}

	pkg := ctx.pkg
	defs := pkg.NewConstDefs(pkg.Types.Scope())
	if doc != nil {
		defs.SetComments(doc)
	}
	for _, m := range members {
		at := defs.NewPos()
		// Attach each member's own leading comment, except the comment that was
		// consumed as the block doc (removed from the macro it came from).
		if md := memberDoc(ctx, m, firstName, lastName, precDoc != nil, trailDoc != nil); md != nil {
			at.Doc = md
		}
		v := m.val
		defs.NewAt(at, func(cb *gogen.CodeBuilder) int {
			cb.Val(v)
			return 1
		}, 0, goNodePos(ctx, m.decl), nil, m.name)
		ctx.macroVals[m.origName] = m.val
	}
}

// memberDoc returns the per-member doc comment for a group member, honoring the
// group-comment removal rules: when the group doc came from the first macro's
// preceding comment, that macro no longer carries a preceding comment; when it
// came from the last macro's trailing comment, that macro no longer carries a
// trailing comment. Members otherwise keep their own preceding/inline/trailing
// comment.
func memberDoc(ctx *pkgCtx, m groupMember, firstName, lastName string, precUsed, trailUsed bool) *ast.CommentGroup {
	if !ctx.keepDoc {
		return nil
	}
	raw := clang.RawCommentsOf(m.decl)
	text := raw.Preceding
	if precUsed && m.origName == firstName {
		text = "" // preceding comment was taken as the group doc
	}
	if text == "" {
		text = raw.Inline
	}
	if text == "" {
		if !(trailUsed && m.origName == lastName) {
			text = raw.Trailing
		}
	}
	if text == "" {
		return nil
	}
	return docCommentGroupRaw(toLineComments(text))
}

// evalMacro evaluates a macro definition to its constant value, returning ok=false
// when it is not a constant expression.
func evalMacro(ctx *pkgCtx, decl clang.Cursor) (v any, ok bool) {
	if oldv, had := ctx.macroVals[clang.String(decl)]; had {
		return oldv, true
	}
	tu := clang.TU(decl)
	tokens, dispose := tu.Tokenize(decl.Extent())
	defer dispose()
	if len(tokens) <= 1 {
		return nil, false
	}
	return evalConstExpr(ctx, tu, tokens[1:])
}
