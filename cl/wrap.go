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
	"bytes"

	"github.com/goplus/gogen"
	"github.com/goplus/lib/c"
	"github.com/goplus/llcppg/clang"
	lc "github.com/llarhub/clang-c"
)

// -----------------------------------------------------------------------------

type WrapFile struct {
	Filename string
	Content  bytes.Buffer
}

var langExts = [...]string{
	LanguageC:   ".c",
	LanguageCXX: ".cpp",
}

func newWrapFile(ctx *pkgCtx) *WrapFile {
	ext := langExts[ctx.lang]
	filename := "_wrap/llcppg" + ext
	llgoFiles := ctx.cflags + ": " + filename
	ctx.llgo.New(func(cb *gogen.CodeBuilder) int {
		cb.Val(llgoFiles)
		return 1
	}, 0, 0, nil, "LLGoFiles")
	return &WrapFile{Filename: filename}
}

func wrapInlineFunc[funcT fnCursor](ctx *pkgCtx, manglingName string, fn funcT, cls *classCtx) string {
	first := ctx.wrap == nil
	if first {
		ctx.wrap = newWrapFile(ctx)
	}
	w := &ctx.wrap.Content
	if first {
		w.WriteString(ctx.wrapFileHeader)
	}
	w.WriteByte('\n')
	wrapName := "_llcppg_" + manglingName
	writeFunc(w, wrapName, fn, cls, ctx.lang)
	return wrapName
}

// -----------------------------------------------------------------------------

type writerT = bytes.Buffer

func writeFunc[funcT fnCursor](b *writerT, name string, fn funcT, cls *classCtx, lang Language) {
	var call writerT
	writeFuncProto(b, &call, name, fn, cls, lang)
	b.WriteString(" {\n")
	b.Write(call.Bytes())
	b.WriteString("}\n")
}

func writeFuncProto[funcT fnCursor](out, call *writerT, name string, fn funcT, cls *classCtx, lang Language) {
	var b writerT
	b.WriteString(name)
	b.WriteByte('(')
	call.WriteByte('\t')
	retType := fn.ResultType()
	if retType.Kind != lc.Type_Void {
		call.WriteString("return ")
	}
	// An instance method takes a "this" receiver and calls "this->method(...)".
	// A static method (loaded as a global function, so cls is nil) has no
	// receiver, but the call must still be qualified as "ClassName::method(...)".
	notFirst := cls != nil
	if cls != nil {
		b.WriteString(clang.String(cls.decl.Type()))
		b.WriteString("* this")
		call.WriteString("this->")
		call.WriteString(fn.Name())
	} else {
		call.WriteString(fn.FullName())
	}
	call.WriteByte('(')
	for i := range fn.NumArguments() {
		if i > 0 {
			call.WriteString(", ")
		}
		if notFirst {
			b.WriteString(", ")
		} else {
			notFirst = true
		}
		argTyp, argName := fn.Argument(i)
		writeParam(&b, argTyp, argName)
		call.WriteString(argName)
	}
	b.WriteByte(')')
	call.WriteString(");\n")
	if lang == LanguageCXX {
		out.WriteString(`extern "C" `)
	}
	writeParam(out, retType, b.String())
}

func writeParam(b *writerT, typ lc.Type, name string) {
	tderef, lvl := deref(typ)
	if tderef.Kind == lc.Type_FunctionProto {
		writeFuncParam(b, tderef, lvl, name)
		return
	}
	b.WriteString(toCType(typ, flagIsParam))
	if typ.Kind != lc.Type_Pointer {
		b.WriteByte(' ')
	}
	b.WriteString(name)
}

func writeFuncParam(out *writerT, fn lc.Type, lvl int, name string) {
	var b writerT
	b.WriteByte('(')
	for range lvl {
		b.WriteByte('*')
	}
	b.WriteString(name)
	b.WriteByte(')')
	b.WriteString("(")
	for i := range c.Uint(fn.NumArgTypes()) {
		if i > 0 {
			b.WriteString(", ")
		}
		arg := fn.Arg(i)
		writeParam(&b, arg, "")
	}
	b.WriteString(")")
	writeParam(out, fn.Result(), b.String())
}

func deref(typ lc.Type) (lc.Type, int) {
	n := 0
	for typ.Kind == lc.Type_Pointer {
		typ = typ.Pointee()
		n++
	}
	return typ, n
}

// -----------------------------------------------------------------------------

func toCType(typ lc.Type, flags int) string {
	_ = flags
	switch typ.Kind {
	case lc.Type_Record:
		return clang.String(typ.Declaration().Type())
	case lc.Type_Elaborated:
		return clang.String(typ.Named())
	default:
		return clang.String(typ)
	}
}

// -----------------------------------------------------------------------------
