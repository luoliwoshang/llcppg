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

package clang

import (
	"github.com/goplus/lib/c"
	"github.com/llarhub/clang-c"
)

// -----------------------------------------------------------------------------

// RawComment returns the raw documentation comment associated with the given
// cursor, including the comment markers (e.g. "/** ... */" or "///"). It returns
// an empty string if the cursor has no associated comment.
func RawComment(entity clang.Cursor) string {
	return GoStringAndDispose(entity.RawCommentText())
}

// MacroRawComment retrieves the raw documentation comment immediately preceding
// a macro definition. libclang does not associate doc comments with macro
// definition cursors (clang_Cursor_getRawCommentText returns "" for them), so
// RawComment is always empty for a #define. This recovers the comment by
// scanning the source text that precedes the macro.
//
// A comment counts as the macro's leading doc only when it sits on the line(s)
// directly above the #define with no blank line in between; a blank line
// detaches it. The returned string keeps the original comment markers so it can
// be fed to the same comment-normalizing logic as RawComment. It returns "" when
// there is no such comment. See MacroTrailingComment for the trailing position
// and issue #1056 for the overall macro attribution order (leading, then
// trailing).
func MacroRawComment(entity clang.Cursor) string {
	var file File
	var offset c.Uint
	entity.Extent().Start().Expansion(&file, nil, nil, &offset)
	if file == InvalidFile {
		return ""
	}
	src := TU(entity).FileContents(file)
	// The macro cursor's extent starts at the macro name, i.e. after the
	// "#define " keyword, so a comment on the preceding line is not directly
	// behind it. Back up to the start of the macro's own line first, then scan
	// for an adjacent comment above it.
	return precedingComment(src, lineStart(src, int(offset)))
}

// MacroTrailingComment retrieves the raw documentation comment on the line
// immediately below a macro definition (the trailing position in issue #1056).
// Like MacroRawComment it scans the source text, since libclang does not
// associate comments with macro cursors.
//
// Macros have no inline comment position, so a comment on the same line as the
// #define is ignored; only a comment that begins on the next line counts, and a
// blank line between the #define and the comment detaches it. The returned
// string keeps the original comment markers. It returns "" when there is no such
// comment.
//
// This is kept separate from MacroRawComment so callers can apply the
// leading-then-trailing order deliberately. Macro functions use both (see
// clangMacroFunc.RawComment); the leading position alone remains the default for
// contexts that have not adopted the trailing position.
func MacroTrailingComment(entity clang.Cursor) string {
	var file File
	var end c.Uint
	rg := entity.Extent()
	rg.Start().Expansion(&file, nil, nil, nil)
	if file == InvalidFile {
		return ""
	}
	src := TU(entity).FileContents(file)
	// The extent ends at the last token of the macro body (after any line
	// continuations), so scanning forward from there reaches the line directly
	// below the whole #define.
	rg.End().Expansion(nil, nil, nil, &end)
	return followingComment(src, int(end))
}

// precedingComment returns the comment block (with markers) that immediately
// precedes the byte at start in src, or "" when none is adjacent. "Adjacent"
// means only whitespace (and at most one newline) separates the comment from
// start, so a blank line between the comment and the declaration detaches it.
func precedingComment(src []byte, start int) string {
	if start > len(src) {
		start = len(src)
	}
	// Walk back over the inline whitespace that sits between the comment and
	// the declaration. Stop at the first non-space byte or the newline that
	// ends the comment's own line; a second newline means a blank line gap,
	// which detaches the comment.
	i := start
	newlines := 0
	for i > 0 {
		c := src[i-1]
		switch c {
		case ' ', '\t', '\r':
			i--
			continue
		case '\n':
			if newlines++; newlines > 1 {
				return ""
			}
			i--
			continue
		}
		break
	}
	if i == 0 {
		return ""
	}

	// A block comment ends with "*/".
	if i >= 2 && src[i-1] == '/' && src[i-2] == '*' {
		for j := i - 2; j >= 1; j-- {
			if src[j-1] == '/' && src[j] == '*' {
				return string(src[j-1 : i])
			}
		}
		return ""
	}

	// Otherwise collect a run of adjacent single-line comments ("//", "///",
	// "//!"). The scan is already positioned at the end of the line holding the
	// nearest comment (newlines <= 1 above guarantees adjacency).
	lineEnd := i
	var first int = -1
	for {
		ls := lineStart(src, lineEnd)
		content := trimLeadingSpace(src[ls:lineEnd])
		if !hasPrefix(content, "//") {
			break
		}
		first = ls
		if ls == 0 {
			break
		}
		lineEnd = ls - 1 // step onto the newline that ends the previous line
		if lineEnd > 0 && src[lineEnd-1] == '\r' {
			lineEnd--
		}
	}
	if first < 0 {
		return ""
	}
	return string(trimLeadingSpace(src[first:i]))
}

// followingComment returns the comment block (with markers) that immediately
// follows the byte at end in src, or "" when none is adjacent. "Adjacent" means
// the comment sits on the line directly below end's line: only the rest of end's
// own line (trailing whitespace) and a single newline may separate them, so a
// blank line between the declaration and the comment detaches it. A comment on
// the same physical line as end is an inline comment, not a trailing one, and is
// ignored (macros have no inline comment position).
func followingComment(src []byte, end int) string {
	if end < 0 {
		end = 0
	}
	// Walk forward over the remainder of the declaration's line, then over at
	// most one newline onto the line below. A second newline means a blank line
	// gap, which detaches any comment.
	i := end
	newlines := 0
	for i < len(src) {
		switch src[i] {
		case ' ', '\t', '\r':
			i++
			continue
		case '\n':
			if newlines++; newlines > 1 {
				return ""
			}
			i++
			continue
		}
		break
	}
	// A comment must start a fresh line to count as trailing; anything still on
	// the declaration's own line (newlines == 0) is inline, which macros ignore.
	if i >= len(src) || newlines == 0 {
		return ""
	}

	// A block comment starts with "/*".
	if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
		for j := i + 2; j+1 < len(src); j++ {
			if src[j] == '*' && src[j+1] == '/' {
				return string(src[i : j+2])
			}
		}
		return ""
	}

	// Otherwise collect a run of adjacent single-line comments ("//", "///",
	// "//!"). Each must begin a line with only whitespace before it.
	if !hasPrefix(trimLeadingSpace(src[i:]), "//") {
		return ""
	}
	first := i
	last := i
	for i < len(src) {
		le := lineEndAt(src, i)
		if !hasPrefix(trimLeadingSpace(src[i:le]), "//") {
			break
		}
		last = le
		// Advance to the start of the next line: skip the trimmed "\r" (if any)
		// and the "\n". If there is no newline, we are at EOF.
		nl := le
		for nl < len(src) && src[nl] != '\n' {
			nl++
		}
		if nl >= len(src) {
			break
		}
		i = nl + 1
	}
	return string(src[first:last])
}

// lineEndAt returns the index of the newline (or len(src)) that terminates the
// line containing pos, trimming a trailing "\r" so CRLF endings are excluded.
func lineEndAt(src []byte, pos int) int {
	for pos < len(src) && src[pos] != '\n' {
		pos++
	}
	if pos > 0 && src[pos-1] == '\r' {
		pos--
	}
	return pos
}

// lineStart returns the index of the first byte of the line containing pos.
func lineStart(src []byte, pos int) int {
	for pos > 0 && src[pos-1] != '\n' {
		pos--
	}
	return pos
}

// trimLeadingSpace drops leading spaces and tabs from b without copying.
func trimLeadingSpace(b []byte) []byte {
	i := 0
	for i < len(b) && (b[i] == ' ' || b[i] == '\t') {
		i++
	}
	return b[i:]
}

// hasPrefix reports whether b starts with the ASCII prefix p.
func hasPrefix(b []byte, p string) bool {
	if len(b) < len(p) {
		return false
	}
	for i := 0; i < len(p); i++ {
		if b[i] != p[i] {
			return false
		}
	}
	return true
}

// -----------------------------------------------------------------------------
