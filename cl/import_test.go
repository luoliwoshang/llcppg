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
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/goplus/gogen"
	"github.com/goplus/gogen/packages"
	"github.com/goplus/llcppg/clang"
	"github.com/qiniu/x/test"
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
				pubDeps: make(map[string]none),
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
			wantDeps := []Entry{{Kind: entryDependency, Name: depPath}}
			if got := p.publicEntries(); !slices.Equal(got, wantDeps) {
				t.Errorf("dependencies with a pub cycle: got %v, want %v", got, wantDeps)
			}
		})
	}
}

func TestWritePubFile(t *testing.T) {
	entries := []Entry{
		{Kind: entryType, Name: "point", GoName: "Point"},
		{Kind: entryDependency, Name: "test/dep"},
	}
	file := filepath.Join(t.TempDir(), "llcppg.pub")
	p := Package{Public: entries}
	if err := p.WritePubFile(file); err != nil {
		t.Fatal(err)
	}
	want := "D test/dep\nT point Point\n"
	if b, err := os.ReadFile(file); err != nil || string(b) != want {
		t.Fatalf("pub contents = %q, %v; want %q", b, err, want)
	}
	if err := (Package{}).WritePubFile(file); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(file); err != nil || len(b) != 0 {
		t.Fatalf("empty pub = %q, %v", b, err)
	}
}

func TestPublicEntriesDeletedType(t *testing.T) {
	pkg := gogen.NewPackage("test/local", "local", nil)
	decl := pkg.NewTypeDefs().NewType("Point", nil)
	obj := decl.InitType(pkg, types.NewStruct(nil, nil)).Obj()
	p := &pkgCtx{
		pkg:      pkg,
		types:    map[string]typeObj{"point": {obj, 0}},
		typdecls: map[string]typDecl{"point": {TypeDecl: decl}},
	}
	p.addPublicType("point", tagStruct)
	want := []Entry{{Kind: entryType, Name: "point", GoName: "Point", Tag: tagStruct}}
	if got := p.publicEntries(); !slices.Equal(got, want) {
		t.Fatalf("before deletion: got %v, want %v", got, want)
	}
	// Delete removes the Go declaration but leaves its type object in scope.
	// The pub must still exclude it, even without an ignored-type flag.
	decl.Delete()
	if got := p.publicEntries(); len(got) != 0 {
		t.Fatalf("deleted type remains in pub: %v", got)
	}
}

const pubFixtureModule = "testpub"

type pubFixtureConfig struct {
	Config
	Deps []string // local Go package paths exposed to PackageOf and PubFileLookup
}

func preparePubFixtures(t *testing.T, dirs ...string) (fixtureRoot, workDir string) {
	t.Helper()
	fixtureRoot, err := filepath.Abs("_testpub")
	if err != nil {
		t.Fatal(err)
	}
	workDir = t.TempDir()
	files := []string{"go.mod", "go.sum"}
	for _, dir := range dirs {
		files = append(files, filepath.Join(dir, "test.h"), filepath.Join(dir, "in.cfg"))
	}
	// Copy only inputs. In particular, A must consume B's newly generated
	// Go source and pub, never the checked-in golden files.
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(fixtureRoot, file))
		if err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(workDir, file)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return
}

func generatePubFixture(t *testing.T, fixtureRoot, workDir, dir string) *types.Package {
	t.Helper()
	pkgDir := filepath.Join(workDir, dir)
	data, err := os.ReadFile(filepath.Join(pkgDir, "in.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg pubFixtureConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	pkgPath := pubFixtureModule + "/" + filepath.ToSlash(dir)
	headers := map[string]string{filepath.Join(pkgDir, "test.h"): pkgPath}
	pubFiles := make(map[string]string)
	for _, dep := range cfg.Deps {
		rel, ok := strings.CutPrefix(dep, pubFixtureModule+"/")
		if !ok {
			t.Fatalf("dependency %q is outside the fixture module", dep)
		}
		depDir := filepath.Join(workDir, filepath.FromSlash(rel))
		headers[filepath.Join(depDir, "test.h")] = dep
		pubFiles[dep] = filepath.Join(depDir, "llcppg.pub")
	}
	conf := &cfg.Config
	conf.Importer = packages.NewImporter(nil, workDir)
	conf.PackageOf = func(header string) (string, bool) {
		pkgPath, ok := headers[filepath.Clean(header)]
		return pkgPath, ok
	}
	conf.PubFileLookup = func(pkgPath string) (string, bool) {
		file, ok := pubFiles[pkgPath]
		return file, ok
	}
	lang := "c"
	if conf.Language == LanguageCXX {
		lang = "c++"
	}
	idx := clang.CreateIndex(1, 1)
	defer idx.Dispose()
	u, err := idx.ParseTranslationUnit(clang.DetailedPreprocessingRecord,
		filepath.Join(pkgDir, "test.h"), "-x", lang)
	if err != nil {
		t.Fatal(err)
	}
	defer u.Dispose()
	pkg, err := NewPackage(pkgPath, filepath.Base(dir), []Source{u}, conf)
	if err != nil {
		t.Fatal(err)
	}
	var goSource bytes.Buffer
	if err := pkg.WriteTo(&goSource); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "out.go"), goSource.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	pubFile := filepath.Join(pkgDir, "llcppg.pub")
	if err := pkg.WritePubFile(pubFile); err != nil {
		t.Fatal(err)
	}
	pub, err := os.ReadFile(pubFile)
	if err != nil {
		t.Fatal(err)
	}
	goldenDir := filepath.Join(fixtureRoot, dir)
	expectedPub, err := os.ReadFile(filepath.Join(goldenDir, "llcppg.pub"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	// Only the pub is compared byte-for-byte to lock its format, including
	// whitespace, entry order and the final newline.
	if test.Diff(t, filepath.Join(goldenDir, "llcppg.pub.txt"), pub, expectedPub) {
		t.Error(goldenDir, ": unexpected pub output")
	}

	// out.go supplies expected exported package-level type names only. These tests
	// focus on pub output; comparing Go source bytes would unnecessarily
	// lock its formatting. Import the generated source and check its scope.
	goGolden := filepath.Join(goldenDir, "out.go")
	expectedGo, err := parser.ParseFile(token.NewFileSet(), goGolden, nil, 0)
	if os.IsNotExist(err) {
		if err := os.WriteFile(goGolden+".txt", goSource.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
		t.Errorf("missing %s; generated source saved to %s.txt", goGolden, goGolden)
	} else if err != nil {
		t.Fatal(err)
	}
	imp := packages.NewImporter(nil, workDir)
	generated, err := imp.Import(pkgPath)
	if err != nil {
		t.Fatal(err)
	}
	if expectedGo != nil {
		for _, decl := range expectedGo.Decls {
			typesDecl, ok := decl.(*ast.GenDecl)
			if !ok || typesDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range typesDecl.Specs {
				name := spec.(*ast.TypeSpec).Name.Name
				if !ast.IsExported(name) {
					continue // imported packages do not expose unused private types
				}
				if _, ok := generated.Scope().Lookup(name).(*types.TypeName); !ok {
					t.Errorf("generated package %s is missing expected type %s", pkgPath, name)
				}
			}
		}
	}

	// Read the newly generated pub and check both raw and tagged C/C++ names
	// against the generated Go source.
	consumer := &pkgCtx{
		pkg:   gogen.NewPackage("testpub/consumer", "consumer", &gogen.Config{Importer: imp}),
		types: make(map[string]typeObj), impPkgs: make(map[string]none), pubDeps: make(map[string]none),
		pubLookup: func(path string) (string, bool) {
			if path == pkgPath {
				return pubFile, true
			}
			return conf.PubFileLookup(path)
		},
	}
	consumer.importPkg(pkgPath)
	for _, e := range pkg.Public {
		if e.Kind != entryType {
			continue
		}
		obj := generated.Scope().Lookup(e.GoName)
		if got := consumer.types[e.Name].TypeName; got != obj {
			t.Errorf("imported %s = %v; want %v", e.Name, got, obj)
		}
		if e.Tag != 0 {
			if got := consumer.types[tagStrvals[e.Tag]+e.Name].TypeName; got != obj {
				t.Errorf("imported tagged %s = %v; want %v", e.Name, got, obj)
			}
		}
	}
	return generated
}

func TestPubGeneration(t *testing.T) {
	for _, dir := range []string{"types", "cxx"} {
		t.Run(dir, func(t *testing.T) {
			fixtureRoot, workDir := preparePubFixtures(t, dir)
			generatePubFixture(t, fixtureRoot, workDir, dir)
		})
	}
}

func TestPubGenerationDependency(t *testing.T) {
	fixtureRoot, workDir := preparePubFixtures(t,
		"basic", "deps/a", "deps/transitive", "deps/direct_a_first", "deps/direct_basic_first")
	generatePubFixture(t, fixtureRoot, workDir, "basic")
	generatePubFixture(t, fixtureRoot, workDir, "deps/a")
	pkg := generatePubFixture(t, fixtureRoot, workDir, "deps/transitive")
	// Direct and transitive fields must reference the dependency's actual
	// Go types despite the consumer's different naming configuration.
	holder := pkg.Scope().Lookup("Holder").Type().Underlying().(*types.Struct)
	for i, want := range []string{
		"*testpub/deps/a.View",
		"*testpub/basic.Point",
		"testpub/basic.Length",
	} {
		if got := types.TypeString(holder.Field(i).Type(), nil); got != want {
			t.Errorf("consumer field %d = %s; want %s", i, got, want)
		}
	}
	// Both include orders must emit the same direct dependency entries.
	for _, dir := range []string{"deps/direct_a_first", "deps/direct_basic_first"} {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			generatePubFixture(t, fixtureRoot, workDir, dir)
		})
	}
}
