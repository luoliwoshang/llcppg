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
	"fmt"
	"go/types"
	"iter"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/goplus/gogen"
	"github.com/goplus/llcppg/clang"
)

// -----------------------------------------------------------------------------

type entryKind = byte

const (
	entryType       entryKind = 'T' // type
	entryFunc       entryKind = 'f' // func
	entryVar        entryKind = 'v' // var
	entryDependency entryKind = 'D' // dependency
)

// Entry represents a C/C++ name and its corresponding Go name.
type Entry struct {
	Name   string
	GoName string    // optional; omitted names use externName, independent of importer configuration
	Tag    typeTag   // only valid for Kind == 'T'
	Kind   entryKind // 'T' (type), 'f' (func), 'v' (var), 'D' (dependency)
}

// -----------------------------------------------------------------------------

// loadPubFile loads a public file and returns an iterator that yields each public entry.
//
// D depPkgPath
// T cName
// T cName goName
// T <tag> cName
// T <tag> cName goName
//
// An omitted Go name uses the default C/C++ name conversion (externName).
// An explicit Go name is used verbatim. Neither depends on importer configuration.
func loadPubFile(pubfile string) (it iter.Seq[Entry], err error) {
	b, err := os.ReadFile(pubfile)
	if err != nil {
		return
	}

	text := string(b)
	lines := strings.Split(text, "\n")
	it = func(yield func(Entry) bool) {
		for i, line := range lines {
			flds := strings.Fields(line)
			switch len(flds) {
			case 0:
				continue
			case 1:
				tooFewOrManyFields(i, "few", line)
			}
			kindFld := flds[0]
			if len(kindFld) != 1 {
				panic(fmt.Errorf("line %d: invalid kind - %s", i+1, kindFld))
			}
			kind := kindFld[0]
			cName := flds[1]
			goName := ""
			tag := typeTag(0)
			if kind == entryType && len(flds) > 2 {
				igo := 2
				if tag = getTypeTag(flds[1]); tag != 0 {
					cName, igo = flds[2], 3
				}
				if igo < len(flds) {
					goName = flds[igo]
				}
			} else if len(flds) > 3 {
				tooFewOrManyFields(i, "many", line)
			}
			if !yield(Entry{Kind: kind, Name: cName, GoName: goName, Tag: tag}) {
				return
			}
		}
	}
	return
}

func getTypeTag(tag string) typeTag {
	switch tag {
	case "enum":
		return tagEnum
	case "struct":
		return tagStruct
	case "class":
		return tagClass
	case "union":
		return tagUnion
	}
	return 0
}

func tooFewOrManyFields(i int, fewOrMany, line string) {
	panic(fmt.Errorf("line %d: too %s fields - %s", i+1, fewOrMany, line))
}

// -----------------------------------------------------------------------------

func (e Entry) pubLine() string {
	line := string(e.Kind) + " "
	if e.Tag != 0 {
		line += tagStrvals[e.Tag]
	}
	line += e.Name
	if e.GoName != "" {
		line += " " + e.GoName
	}
	return line
}

// -----------------------------------------------------------------------------

func publicTypeTag(decl clang.Cursor) typeTag {
	switch decl.Kind {
	case tagStruct, tagClass, tagUnion, tagEnum:
	default:
		return 0 // templates have no distinct tag in the pub format
	}
	// Clang borrows the typedef name for an anonymous tag, e.g.
	// typedef struct { int x; } Point. Its spelling range still points to
	// the tag keyword; "struct Point" is not a declared C type name.
	u := clang.TU(decl)
	tokens, dispose := u.Tokenize(decl.SpellingNameRange(0, 0))
	defer dispose()
	if len(tokens) != 0 && getTypeTag(u.TokenSpelling(tokens[0])) != 0 {
		return 0
	}
	return decl.Kind
}

func (p *pkgCtx) addPublicType(cName string, tag typeTag) {
	p.pubs = append(p.pubs, Entry{Kind: entryType, Name: cName, Tag: tag})
}

// publicEntries resolves candidates against the final package. A type can be
// registered before its definition fails, or be an internal type parameter
// rather than a package-level declaration; neither should appear in the pub.
func (p *pkgCtx) publicEntries() []Entry {
	pkg := p.pkg.Types
	scope := pkg.Scope()
	entries := make(map[Entry]none)
	for _, e := range p.pubs {
		o := p.types[e.Name]
		if o.TypeName == nil || o.feats&(featAllIgnore|featTyIgnore) != 0 ||
			o.Pkg() != pkg || !o.Exported() || scope.Lookup(o.Name()) != o.TypeName {
			continue
		}
		if decl, ok := p.typdecls[e.Name]; ok && decl.State() == gogen.TyStateDeleted {
			continue
		}
		e.GoName = o.Name()
		entries[e] = none{}
	}
	ret := make([]Entry, 0, len(entries))
	for e := range entries {
		ret = append(ret, e)
	}
	slices.SortFunc(ret, func(a, b Entry) int {
		return strings.Compare(a.pubLine(), b.pubLine())
	})
	return ret
}

// -----------------------------------------------------------------------------

func (p *pkgCtx) forceImportUnsafe() {
	p.pkg.ForceImport("unsafe")
}

func (p *pkgCtx) importPkg(pkgPath string) *types.Package {
	if _, ok := p.impPkgs[pkgPath]; ok {
		return nil // imported already
	}
	p.impPkgs[pkgPath] = none{}

	pubFile, ok := p.pubLookup(pkgPath)
	if !ok {
		log.Panicln("[ERROR] pubFile not found for", pkgPath)
	}

	if debugMajorProc {
		log.Println("==> importPkg", pkgPath)
	}

	pkg := p.pkg.Import(pkgPath)
	entries, err := loadPubFile(pubFile)
	if err != nil {
		if os.IsNotExist(err) {
			return pkg.Types // ignore missing pub file
		}
		log.Panicln("[ERROR] loadPubFile failed:", err)
	}

	scope := pkg.Types.Scope()
	for e := range entries {
		switch e.Kind {
		case entryDependency:
			p.importPkg(e.Name)

		case entryType:
			if e.GoName == "" {
				e.GoName = externName(e.Name)
			}
			if o := scope.Lookup(e.GoName); o != nil {
				if t, ok := o.(*types.TypeName); ok {
					p.types[e.Name] = typeObj{t, 0}
					if e.Tag != 0 {
						p.types[tagStrvals[e.Tag]+e.Name] = typeObj{t, 0}
					}
				}
			}

		default:
			panic("importPkg: unsupport - " + e.Name)
		}
	}
	return pkg.Types
}

// -----------------------------------------------------------------------------
