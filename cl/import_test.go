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
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"github.com/goplus/gogen"
)

type pubTestImporter map[string]*types.Package

func (p pubTestImporter) Import(path string) (*types.Package, error) {
	if pkg, ok := p[path]; ok {
		return pkg, nil
	}
	return nil, fmt.Errorf("unexpected import: %s", path)
}

func TestImportPubNames(t *testing.T) {
	const depPath, transitivePath = "test/dep", "test/transitive"
	dep := types.NewPackage(depPath, "dep")
	transitive := types.NewPackage(transitivePath, "transitive")
	for _, name := range []string{"SizeT", "Color", "SomeUnion", "SomeEnum", "NsWidget", "Foo_Bar", "Widget"} {
		obj := types.NewTypeName(token.NoPos, dep, name, nil)
		types.NewNamed(obj, types.Typ[types.Int], nil)
		dep.Scope().Insert(obj)
	}
	// The reported failure involves aliases such as cstdlib.TimeT.
	transitive.Scope().Insert(types.NewTypeName(token.NoPos, transitive, "TimeT", types.Typ[types.Int64]))
	dep.MarkComplete()
	transitive.MarkComplete()

	dir := t.TempDir()
	pubs := map[string]string{
		depPath:        "D test/transitive\nT size_t\nT struct Color\nT union SomeUnion\nT enum SomeEnum\nT class ns::widget\nT Foo_Bar\nT b_widget_t Widget\nT struct special_t Foo_Bar\n",
		transitivePath: "D test/dep\nT time_t\n",
	}
	pubFiles := make(map[string]string)
	for path, content := range pubs {
		file := filepath.Join(dir, filepath.Base(path)+".pub")
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		pubFiles[path] = file
	}

	tests := []struct {
		name  string
		setup func(*pkgCtx)
	}{
		{"default", func(p *pkgCtx) {}},
		{"type_suffix", func(p *pkgCtx) { p.typeSuffix = []string{"_t"} }},
		{"type_prefix", func(p *pkgCtx) { p.typePrefix = []string{"size_", "time_"} }},
		{"namespace_prefix", func(p *pkgCtx) { p.nsPrefix = []string{"ns::"} }},
		{"rename_full", func(p *pkgCtx) {
			p.rename["size_t"] = "LocalSize"
			p.rename["b_widget_t"] = "LocalWidget"
			p.rename["special_t"] = "LocalSpecial"
		}},
		{"rename_parts", func(p *pkgCtx) {
			p.rename["size"] = "LocalSize"
			p.rename["t"] = "LocalT"
		}},
		{"force_camel_case", func(p *pkgCtx) { p.forceCamelCase = true }},
		{"local_alias", func(p *pkgCtx) { p.aliasTypeName("time_t", "LocalTime") }},
		{"combined", func(p *pkgCtx) {
			p.nsPrefix = []string{"ns::"}
			p.typePrefix = []string{"size_"}
			p.typeSuffix = []string{"_t"}
			p.rename["time_t"] = "LocalTime"
			p.rename["widget"] = "LocalWidget"
			p.forceCamelCase = true
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &pkgCtx{
				pkg: gogen.NewPackage("test/importer", "importer", &gogen.Config{
					Importer: pubTestImporter{depPath: dep, transitivePath: transitive},
				}),
				rename:  make(map[string]string),
				types:   make(map[string]typeObj),
				impPkgs: make(map[string]none),
				pubLookup: func(path string) (string, bool) {
					file, ok := pubFiles[path]
					return file, ok
				},
			}
			tt.setup(p)
			if got := p.importPkg(depPath); got != dep {
				t.Fatalf("importPkg returned %v, want dependency package", got)
			}
			for cName, goName := range map[string]string{
				"size_t": "SizeT", "Color": "Color", "struct Color": "Color",
				"SomeUnion": "SomeUnion", "union SomeUnion": "SomeUnion",
				"SomeEnum": "SomeEnum", "enum SomeEnum": "SomeEnum",
				"ns::widget": "NsWidget", "class ns::widget": "NsWidget",
				"Foo_Bar": "Foo_Bar", "b_widget_t": "Widget",
				"special_t": "Foo_Bar", "struct special_t": "Foo_Bar",
			} {
				if got := p.types[cName].TypeName; got != dep.Scope().Lookup(goName) {
					t.Errorf("mapping %q: got %v, want dep.%s", cName, got, goName)
				}
			}
			if got := p.types["time_t"].TypeName; got != transitive.Scope().Lookup("TimeT") {
				t.Errorf("transitive time_t mapping: got %v, want transitive.TimeT", got)
			}
		})
	}
}
