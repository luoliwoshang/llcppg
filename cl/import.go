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
	GoName string    // optional; omitted names use externName, independent of llcppg.cfg
	Kind   entryKind // 'T' (type), 'f' (func), 'v' (var), 'D' (dependency)
}

func (e Entry) pubLine() string {
	line := string(e.Kind) + " " + e.Name
	if e.GoName != "" {
		line += " " + e.GoName
	}
	return line
}

// -----------------------------------------------------------------------------

// loadPubFile loads a public file and returns an iterator that yields each public entry.
//
// D depPkgPath
// T cName
// T cName goName
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
			if kind == entryType && len(flds) == 3 {
				goName = flds[2]
			} else if len(flds) > 2 {
				tooFewOrManyFields(i, "many", line)
			}
			if !yield(Entry{Kind: kind, Name: cName, GoName: goName}) {
				return
			}
		}
	}
	return
}

func tooFewOrManyFields(i int, fewOrMany, line string) {
	panic(fmt.Errorf("line %d: too %s fields - %s", i+1, fewOrMany, line))
}

// -----------------------------------------------------------------------------

// PubFile generates the byte representation of the public entries, sorted by their
// serialized lines.
func PubFile(entries []Entry) []byte {
	size := 0
	lines := make([]string, len(entries))
	for i, e := range entries {
		line := e.pubLine()
		lines[i] = line
		size += len(line) + 1
	}
	slices.Sort(lines)
	b := make([]byte, 0, size)
	for _, line := range lines {
		b = append(b, line...)
		b = append(b, '\n')
	}
	return b
}

// -----------------------------------------------------------------------------

func (p *pkgCtx) addPublicType(cName string) {
	p.pubTyps[cName] = none{}
}

// publicEntries resolves candidates against the final package. A type can be
// registered before its definition fails, or be an internal type parameter
// rather than a package-level declaration; neither should appear in the pub.
func (p *pkgCtx) publicEntries() []Entry {
	pkg := p.pkg.Types
	scope := pkg.Scope()
	ret := make([]Entry, 0, len(p.pubDeps)+len(p.pubTyps))
	for dep := range p.pubDeps {
		_, implicit := p.implicitPkgs[dep]
		if !implicit {
			ret = append(ret, Entry{Kind: entryDependency, Name: dep})
		}
	}
	for name := range p.pubTyps {
		o := p.types[name]
		if o.TypeName == nil || o.feats&(featAllIgnore|featTyIgnore) != 0 ||
			o.Pkg() != pkg || !o.Exported() || scope.Lookup(o.Name()) != o.TypeName {
			continue
		}
		if decl, ok := p.typdecls[name]; ok && decl.State() == gogen.TyStateDeleted {
			continue
		}
		ret = append(ret, Entry{Kind: entryType, Name: name, GoName: o.Name()})
	}
	return ret
}

// -----------------------------------------------------------------------------

func (p *pkgCtx) forceImportUnsafe() {
	p.pkg.ForceImport("unsafe")
}

func (p *pkgCtx) importPkg(pkgPath string) *types.Package {
	// Record the direct dependency even if it was already loaded through
	// another package's pub. Dependency output must not depend on load order.
	p.pubDeps[pkgPath] = none{}
	return p.loadPkg(pkgPath)
}

// loadPkg loads pub mappings recursively without adding direct dependencies.
func (p *pkgCtx) loadPkg(pkgPath string) *types.Package {
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
			p.loadPkg(e.Name)

		case entryType:
			if e.GoName == "" {
				e.GoName = externName(e.Name)
			}
			if o := scope.Lookup(e.GoName); o != nil {
				if t, ok := o.(*types.TypeName); ok {
					p.types[e.Name] = typeObj{t, 0}
				}
			}

		default:
			panic("importPkg: unsupport - " + e.Name)
		}
	}
	return pkg.Types
}

// -----------------------------------------------------------------------------
