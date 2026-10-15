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
	"go/types"

	"github.com/goplus/lib/c"
	"github.com/goplus/llcppg/clang"
	lc "github.com/llarhub/clang-c"
)

// -----------------------------------------------------------------------------

// macroFuncParam describes one parameter of a function-like macro listed in
// MacroFunc, together with the reference-function parameter whose type it uses.
type macroFuncParam struct {
	name    string  // the macro's own parameter name
	refType lc.Type // the type supplied by the matched reference-function parameter
}

// loadMacroFunc handles a function-like macro that appears in the MacroFunc
// config. It parses the macro's parameter names and defers the rest of the work
// (resolving the reference function and emitting the wrapper + Go declaration)
// to a compile unit, so that all function declarations are already registered
// regardless of whether they precede or follow the macro in the headers.
func loadMacroFunc(ctx *pkgCtx, decl clang.Cursor, refName string) {
	macroName := clang.String(decl)
	paramNames, variadic, ok := macroParamNames(decl)
	if !ok {
		return
	}
	if variadic {
		ctx.errorf(decl, "MacroFunc %s: variadic macros are not supported", macroName)
		return
	}
	ctx.addCompileUnit(func(ctx *pkgCtx) {
		compileMacroFunc(ctx, decl, macroName, refName, paramNames)
	})
}

// macroParamNames parses the parameter list of a function-like macro from its
// tokens. For "#define z_gzgetc(g) ..." the tokens are "z_gzgetc", "(", "g",
// ")", ... so the parameters are the identifiers between the first "(" (which
// immediately follows the macro name for a function-like macro) and the
// matching ")".
func macroParamNames(decl clang.Cursor) (names []string, variadic, ok bool) {
	tu := clang.TU(decl)
	tokens, dispose := tu.Tokenize(decl.Extent())
	defer dispose()
	// tokens[0] is the macro name; tokens[1] must be "(" for a function-like
	// macro. Walk until the matching ")".
	if len(tokens) < 3 || tu.TokenSpelling(tokens[1]) != "(" {
		return
	}
	for i := 2; i < len(tokens); i++ {
		spelling := tu.TokenSpelling(tokens[i])
		switch spelling {
		case ")":
			return names, variadic, true
		case ",":
			// separator, skip
		case "...":
			variadic = true
		default:
			if tokens[i].Kind() == lc.Token_Identifier {
				names = append(names, spelling)
			}
		}
	}
	return
}

// compileMacroFunc resolves the reference function, matches the macro's
// parameters to it, emits the C wrapper, and generates the Go declaration.
func compileMacroFunc(ctx *pkgCtx, decl clang.Cursor, macroName, refName string, paramNames []string) {
	ref, ok := ctx.lookupFuncDecl(refName)
	if !ok {
		ctx.errorf(decl, "MacroFunc %s: reference function %s not found", macroName, refName)
		return
	}
	if ref.IsVariadic() != 0 {
		ctx.errorf(decl, "MacroFunc %s: reference function %s is variadic, not supported", macroName, refName)
		return
	}

	params, ok := matchMacroParams(ctx, decl, macroName, refName, paramNames, ref)
	if !ok {
		return
	}

	manglingName := wrapMacroFunc(ctx, macroName, params, ref)

	pkg := ctx.pkg
	pkgTypes := pkg.Types

	feats := 0
	goParams := make([]*types.Var, len(params))
	for i, p := range params {
		typ := toTypeEx(ctx, pkgTypes, p.refType, flagIsParam, &feats, nil)
		goParams[i] = types.NewParam(goNodePos(ctx, decl), pkgTypes, avoidKeyword(p.name), typ)
	}
	results := toFuncResults(ctx, pkgTypes, ref.ResultType(), &feats, nil)
	if feats&featAllIgnore != 0 {
		ctx.ignoref(feats, decl, "MacroFunc %s: unsupported features, ignored", macroName)
		return
	}

	fnName := ctx.funcName(macroName, -1, "", "", true, true)
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(goParams...), results, false)
	f, err := pkg.NewFuncWith(goNodePos(ctx, decl), fnName, sig, nil)
	if err != nil {
		ctx.errorf(decl, "%v", err)
		return
	}
	ctx.forceImportUnsafe()
	f.SetComments(pkg, ctx.macroFuncComments(decl, ref,
		"\n//go:linkname "+fnName+" C."+manglingName))
}

// macroFuncComments builds the doc comment for a wrapped macro: the macro's own
// doc comment takes precedence, falling back to the reference function's, and
// then to the generated directive alone. The two sources are never merged.
func (p *pkgCtx) macroFuncComments(macro, ref clang.Cursor, directive string) *ast.CommentGroup {
	if p.keepDoc && clang.RawComment(macro) == "" && clang.RawComment(ref) != "" {
		return p.directiveComments(ref, directive)
	}
	return p.directiveComments(macro, directive)
}

// lookupFuncDecl finds a declared global function cursor by its C name among the
// parsed declarations.
func (p *pkgCtx) lookupFuncDecl(cName string) (clang.Cursor, bool) {
	if ovs, ok := p.overloads[cName]; ok {
		for _, obj := range ovs.items {
			if obj.decl.Kind == lc.Cursor_FunctionDecl {
				return obj.decl, true
			}
		}
	}
	return clang.Cursor{}, false
}

// matchMacroParams resolves, for every macro parameter, which reference-function
// parameter supplies its type. Name matches are resolved first for all
// parameters; the remaining parameters fall back to position, skipping any
// reference parameter already claimed by a name match.
func matchMacroParams(ctx *pkgCtx, decl clang.Cursor, macroName, refName string, paramNames []string, ref clang.Cursor) ([]macroFuncParam, bool) {
	nRef := int(ref.NumArguments())
	refArgs := make([]clang.Cursor, nRef)
	refByName := make(map[string]int, nRef)
	for i := range nRef {
		arg := ref.Argument(c.Uint(i))
		refArgs[i] = arg
		if name := clang.String(arg); name != "" {
			refByName[name] = i
		}
	}

	params := make([]macroFuncParam, len(paramNames))
	claimed := make([]bool, nRef)
	resolved := make([]bool, len(paramNames))

	// 1) match by name.
	for i, name := range paramNames {
		params[i].name = name
		if j, ok := refByName[name]; ok && !claimed[j] {
			params[i].refType = refArgs[j].Type()
			claimed[j] = true
			resolved[i] = true
		}
	}

	// 2) match remaining by position, skipping claimed reference parameters.
	for i := range paramNames {
		if resolved[i] {
			continue
		}
		if i < nRef && !claimed[i] {
			params[i].refType = refArgs[i].Type()
			claimed[i] = true
			resolved[i] = true
			continue
		}
		ctx.errorf(decl, "MacroFunc %s: cannot resolve parameter %s from reference function %s",
			macroName, paramNames[i], refName)
		return nil, false
	}
	return params, true
}

// wrapMacroFunc emits a C wrapper function whose body invokes the macro, into
// the shared wrapper output used by inline functions, and returns the wrapper's
// symbol name. The wrapper name differs from the macro name so defining it is
// not macro-expanded.
func wrapMacroFunc(ctx *pkgCtx, macroName string, params []macroFuncParam, ref clang.Cursor) string {
	first := ctx.wrap == nil
	if first {
		ctx.wrap = newWrapFile(ctx)
	}
	w := &ctx.wrap.Content
	if first {
		w.WriteString(ctx.wrapFileHeader)
	}
	w.WriteByte('\n')
	wrapName := "_llcppg_" + macroName
	writeMacroFunc(w, wrapName, macroName, params, ref, ctx.lang)
	return wrapName
}

func writeMacroFunc(b *writerT, wrapName, macroName string, params []macroFuncParam, ref clang.Cursor, lang Language) {
	var proto, call writerT
	proto.WriteString(wrapName)
	proto.WriteByte('(')
	call.WriteByte('\t')
	retVoid := ref.ResultType().Kind == lc.Type_Void
	if !retVoid {
		call.WriteString("return ")
	}
	call.WriteString(macroName)
	call.WriteByte('(')
	for i, p := range params {
		if i > 0 {
			proto.WriteString(", ")
			call.WriteString(", ")
		}
		writeParam(&proto, p.refType, p.name)
		call.WriteString(p.name)
	}
	proto.WriteByte(')')
	call.WriteString(");\n")

	if lang == LanguageCXX {
		b.WriteString(`extern "C" `)
	}
	writeParam(b, ref.ResultType(), proto.String())
	b.WriteString(" {\n")
	b.Write(call.Bytes())
	b.WriteString("}\n")
}

// -----------------------------------------------------------------------------
