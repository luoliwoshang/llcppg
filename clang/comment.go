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
// A comment counts as the macro's doc only when it sits on the line(s) directly
// above the #define with no blank line in between; a blank line detaches it. The
// returned string keeps the original comment markers so it can be fed to the
// same comment-normalizing logic as RawComment. It returns "" when there is no
// such comment.
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
