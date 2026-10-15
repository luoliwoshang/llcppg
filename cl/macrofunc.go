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
	"github.com/goplus/lib/c"
	"github.com/goplus/llcppg/clang"
	lc "github.com/llarhub/clang-c"
)

// -----------------------------------------------------------------------------

func loadMacroFunc(ctx *pkgCtx, decl clang.Cursor, macroName, refName string) {
	ctx.addCompileUnit(func(ctx *pkgCtx) {
		compileMacroFunc(ctx, decl, macroName, refName)
	})
}

// -----------------------------------------------------------------------------

// compileMacroFunc resolves the reference function, matches the macro's
// parameters to it, emits the C wrapper, and generates the Go declaration.
func compileMacroFunc(ctx *pkgCtx, decl clang.Cursor, macroName, refName string) {
	paramNames, variadic, ok := macroParamNames(decl)
	if !ok {
		return
	}
	if variadic {
		ctx.errorf(decl, "macroFunc %s: variadic macros are not supported", macroName)
		return
	}

	ref, ok := ctx.lookupFuncDecl(refName)
	if !ok {
		ctx.errorf(decl, "macroFunc %s: reference function %s not found", macroName, refName)
		return
	}

	params, ok := matchMacroParams(ctx, decl, macroName, refName, paramNames, ref)
	if !ok {
		return
	}

	fn := &clangMacroFunc{
		Cursor: decl,
		params: params,
		ref:    ref,
	}
	doCompileFunc(ctx, macroName, macroName, fn, &overloadObj{name: macroName}, nil)
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
			ok = true
			return
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

// macroFuncParam describes one parameter of a function-like macro listed in
// MacroFunc, together with the reference-function parameter whose type it uses.
type macroFuncParam struct {
	name string  // the macro's own parameter name
	typ  lc.Type // the type supplied by the matched reference-function parameter
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
			params[i].typ = refArgs[j].Type()
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
			params[i].typ = refArgs[i].Type()
			claimed[i] = true
			resolved[i] = true
			continue
		}
		ctx.errorf(decl, "macroFunc %s: cannot resolve parameter %s from reference function %s",
			macroName, paramNames[i], refName)
		return nil, false
	}
	return params, true
}

// -----------------------------------------------------------------------------

type clangMacroFunc struct {
	clangNode
	params []macroFuncParam
	ref    clang.Cursor
}

// RawComment returns the macro's own documentation comment, resolved in the
// order defined by issue #1056: a leading comment above the #define, otherwise a
// trailing comment on the line below it (macros have no inline position). A
// macro does not inherit the comment of the function it references: that comment
// documents a different declaration, so attaching it here would misattribute it.
func (fn *clangMacroFunc) RawComment() string {
	if raw := clang.MacroRawComment(fn.Cursor); raw != "" {
		return raw
	}
	return clang.MacroTrailingComment(fn.Cursor)
}

func (fn *clangMacroFunc) NumArguments() c.Int {
	return c.Int(len(fn.params))
}

func (fn *clangMacroFunc) ArgumentType(i c.Int) lc.Type {
	return fn.params[i].typ
}

func (fn *clangMacroFunc) Argument(i c.Int) (typ lc.Type, name string) {
	param := fn.params[i]
	return param.typ, param.name
}

func (fn *clangMacroFunc) ResultType() lc.Type {
	return fn.ref.ResultType()
}

func (fn *clangMacroFunc) IsVariadic() c.Uint {
	return 0 // not variadic
}

func (fn *clangMacroFunc) IsFunctionInlined() c.Uint {
	return 1 // always inlined
}

// -----------------------------------------------------------------------------
