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
		{Kind: entryDependency, Name: "test/dep"},
		{Kind: entryType, Name: "ns::Widget", GoName: "Widget", Tag: tagClass},
		{Kind: entryType, Name: "status", GoName: "Status", Tag: tagEnum},
		{Kind: entryType, Name: "size_t", GoName: "SizeT"},
		{Kind: entryType, Name: "point", GoName: "Point", Tag: tagStruct},
		{Kind: entryType, Name: "value", GoName: "Value", Tag: tagUnion},
	}
	file := filepath.Join(t.TempDir(), "llcppg.pub")
	p := Package{Public: entries}
	if err := p.WritePubFile(file); err != nil {
		t.Fatal(err)
	}
	want := "D test/dep\nT class ns::Widget Widget\nT enum status Status\nT size_t SizeT\nT struct point Point\nT union value Value\n"
	if b, err := os.ReadFile(file); err != nil || string(b) != want {
		t.Fatalf("pub contents = %q, %v; want %q", b, err, want)
	}
	it, err := loadPubFile(file)
	if err != nil {
		t.Fatal(err)
	}
	got := slices.Collect(it)
	if !slices.Equal(got, entries) {
		t.Fatalf("round trip = %v; want %v", got, entries)
	}

	if err := (Package{}).WritePubFile(file); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(file); err != nil || len(b) != 0 {
		t.Fatalf("empty pub = %q, %v", b, err)
	}
}

func compilePubHeader(t *testing.T, idx clang.Index, path, pkgPath string, conf *Config) Package {
	t.Helper()
	lang := "c"
	if conf.Language == LanguageCXX {
		lang = "c++"
	}
	u, err := idx.ParseTranslationUnit(clang.DetailedPreprocessingRecord, path, "-x", lang)
	if err != nil {
		t.Fatal(err)
	}
	defer u.Dispose()
	if conf.Importer == nil {
		conf.Importer = packages.NewImporter(nil, ".")
	}
	pkg, err := NewPackage(pkgPath, "generated", []Source{u}, conf)
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

func TestGeneratedPublicTypes(t *testing.T) {
	idx := clang.CreateIndex(1, 1)
	defer idx.Dispose()
	for _, tt := range []struct {
		name   string
		lang   Language
		header string
		want   string
	}{
		{
			name: "c",
			header: `
typedef unsigned long b_size_t;
typedef int b_count_t;
typedef int b_count_t;
#define POINT_TAG b_macro_t
struct POINT_TAG { int x; };
#undef POINT_TAG
#define ANON_STRUCT struct
typedef ANON_STRUCT { int x; } b_macro_anon_t;
struct b_point_t { int x; int y; };
typedef struct b_point_t b_point_t;
typedef struct b_point_t b_alias_t;
typedef struct { int x; } b_anon_t;
enum b_status_t { B_OK };
union b_value_t { int n; float f; };
struct b_handle_t;
typedef struct b_handle_t *b_handle_ptr_t;
struct b_container_t { struct { int x; } inner; };
typedef void (*b_callback_t)(int);
typedef int b_configured_t;
typedef void b_void_t;
typedef int b_private_t;
typedef int b_ignored_t;
typedef _Complex float b_bad_t;
struct b_rejected_t { _Complex float unsupported; };
`,
			want: "T b_alias_t Alias\nT b_anon_t Anon\nT b_callback_t Callback\nT b_configured_t Configured\nT b_count_t Count\nT b_handle_ptr_t HandlePtr\nT b_macro_anon_t MacroAnon\nT b_point_t Point\nT b_size_t Size\nT enum b_status_t Status\nT struct b_container_t Container\nT struct b_handle_t Handle\nT struct b_macro_t Macro\nT struct b_point_t Point\nT union b_value_t Value\n",
		},
		{
			name: "naming_config",
			header: `
typedef int b_Open_Foo_t;
typedef int b_widget_t;
`,
			want: "T b_Open_Foo_t OpenFoo\nT b_widget_t CustomWidget\n",
		},
		{
			name: "cxx",
			lang: LanguageCXX,
			header: `
namespace api {
class b_widget_t { int x; };
struct b_record_t { int x; };
union b_value_t { int x; float y; };
enum class b_color_t { red, green };
using b_index_t = int;
template <typename T> struct b_box_t {
    typedef T element_type;
    T value;
};
template <typename T> using b_ptr_t = T*;
}
`,
			want: "T api::b_box_t Box\nT api::b_index_t Index\nT class api::b_widget_t Widget\nT enum api::b_color_t Color\nT struct api::b_record_t Record\nT union api::b_value_t Value\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			header := filepath.Join(dir, "in.h")
			if err := os.WriteFile(header, []byte(tt.header), 0600); err != nil {
				t.Fatal(err)
			}
			pkg := compilePubHeader(t, idx, header, "test/generated", &Config{
				Language:       tt.lang,
				NSPrefix:       []string{"api::"},
				TypePrefix:     []string{"b_"},
				TypeSuffix:     []string{"_t"},
				ForceCamelCase: true,
				Rename:         map[string]string{"b_private_t": "private", "b_widget_t": "CustomWidget"},
				TypeIgnore:     []string{"b_ignored_t"},
				TypeAlias:      map[string]string{"b_configured_t": "c.Long", "b_void_t": "void"},
			})
			file := filepath.Join(dir, "llcppg.pub")
			if err := pkg.WritePubFile(file); err != nil {
				t.Fatal(err)
			}
			if b, err := os.ReadFile(file); err != nil || string(b) != tt.want {
				t.Fatalf("pub = %q, %v; want %q", b, err, tt.want)
			}
			// Read what was generated using an importer with different naming
			// settings, and verify both the raw and tagged C names resolve.
			consumer := &pkgCtx{
				pkg: gogen.NewPackage("test/consumer", "consumer", &gogen.Config{
					Importer: pubTestImporter{pkg.Types.Path(): pkg.Types},
				}),
				types:      make(map[string]typeObj),
				impPkgs:    make(map[string]none),
				pubDeps:    make(map[string]none),
				rename:     map[string]string{"b_size_t": "ConsumerSize"},
				typeSuffix: []string{"_t"},
				pubLookup:  func(string) (string, bool) { return file, true },
			}
			consumer.importPkg(pkg.Types.Path())
			for _, e := range pkg.Public {
				obj := pkg.Types.Scope().Lookup(e.GoName)
				if got := consumer.types[e.Name].TypeName; got != obj {
					t.Errorf("imported %s = %v; want %v", e.Name, got, obj)
				}
				if e.Tag != 0 {
					if got := consumer.types[tagStrvals[e.Tag]+e.Name].TypeName; got != obj {
						t.Errorf("imported tagged %s = %v; want %v", e.Name, got, obj)
					}
				}
			}
		})
	}
}

func TestGeneratedPubDependencies(t *testing.T) {
	idx := clang.CreateIndex(1, 1)
	defer idx.Dispose()
	dir := t.TempDir()
	depHeader := filepath.Join(dir, "dep.h")
	bHeader := filepath.Join(dir, "b.h")
	aHeader := filepath.Join(dir, "a.h")
	for name, content := range map[string]string{
		depHeader: "#pragma once\ntypedef unsigned long ext_value_t;\nstruct ext_point { int x; };\n",
		bHeader:   "#pragma once\n#include \"dep.h\"\ntypedef ext_value_t b_value_t;\nstruct b_holder_t { struct ext_point *p; b_value_t n; };\n",
		aHeader:   "#include \"b.h\"\nstruct a_holder_t { struct b_holder_t *p; struct ext_point *q; ext_value_t n; };\n",
	} {
		if err := os.WriteFile(name, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	imp := packages.NewImporter(nil, ".")
	dep := compilePubHeader(t, idx, depHeader, "test/dep", &Config{
		Importer: imp, TypePrefix: []string{"ext_"}, TypeSuffix: []string{"_t"},
	})
	depPub := filepath.Join(dir, "dep.pub")
	if err := dep.WritePubFile(depPub); err != nil {
		t.Fatal(err)
	}
	libc, err := imp.Import(libcPkgPath)
	if err != nil {
		t.Fatal(err)
	}
	pubFiles := map[string]string{"test/dep": depPub, "test/b": filepath.Join(dir, "b.pub")}
	lookup := func(path string) (string, bool) {
		file, ok := pubFiles[path]
		return file, ok
	}
	packageOf := func(header string) (string, bool) {
		switch header {
		case depHeader:
			return "test/dep", true
		case bHeader:
			return "test/b", true
		case aHeader:
			return "test/a", true
		}
		return "", false
	}
	b := compilePubHeader(t, idx, bHeader, "test/b", &Config{
		Importer:   pubTestImporter{libcPkgPath: libc, "unsafe": types.Unsafe, "test/dep": dep.Types},
		TypePrefix: []string{"b_"}, TypeSuffix: []string{"_t"},
		PackageOf: packageOf, PubFileLookup: lookup,
	})
	if err := b.WritePubFile(pubFiles["test/b"]); err != nil {
		t.Fatal(err)
	}
	const wantB = "D test/dep\nT b_value_t Value\nT struct b_holder_t Holder\n"
	if content, err := os.ReadFile(pubFiles["test/b"]); err != nil || string(content) != wantB {
		t.Fatalf("B pub = %q, %v; want %q", content, err, wantB)
	}
	a := compilePubHeader(t, idx, aHeader, "test/a", &Config{
		Importer:   pubTestImporter{libcPkgPath: libc, "unsafe": types.Unsafe, "test/dep": dep.Types, "test/b": b.Types},
		TypePrefix: []string{"a_"}, TypeSuffix: []string{"_t"},
		Rename:    map[string]string{"ext_value_t": "ConsumerValue", "b_holder_t": "ConsumerHolder"},
		PackageOf: packageOf, PubFileLookup: lookup,
	})
	// Both the direct and transitive types must refer to the dependency's
	// actual Go objects despite A's different naming configuration.
	holder := a.Types.Scope().Lookup("Holder").Type().Underlying().(*types.Struct)
	for i, want := range []types.Type{
		types.NewPointer(b.Types.Scope().Lookup("Holder").Type()),
		types.NewPointer(dep.Types.Scope().Lookup("Point").Type()),
		dep.Types.Scope().Lookup("Value").Type(),
	} {
		if got := holder.Field(i).Type(); !types.Identical(got, want) {
			t.Errorf("A field %d = %v; want %v", i, got, want)
		}
	}
	aPub := filepath.Join(dir, "a.pub")
	if err := a.WritePubFile(aPub); err != nil {
		t.Fatal(err)
	}
	const wantA = "D test/b\nT struct a_holder_t Holder\n"
	if content, err := os.ReadFile(aPub); err != nil || string(content) != wantA {
		t.Fatalf("A pub = %q, %v; want %q", content, err, wantA)
	}
	// A can also include the transitive dependency directly. Record that
	// direct import regardless of whether B loaded it first.
	for _, includes := range []string{
		"#include \"b.h\"\n#include \"dep.h\"\n",
		"#include \"dep.h\"\n#include \"b.h\"\n",
	} {
		if err := os.WriteFile(aHeader, []byte(includes), 0600); err != nil {
			t.Fatal(err)
		}
		pkg := compilePubHeader(t, idx, aHeader, "test/a", &Config{
			Importer:  pubTestImporter{libcPkgPath: libc, "test/dep": dep.Types, "test/b": b.Types},
			PackageOf: packageOf, PubFileLookup: lookup,
		})
		if err := pkg.WritePubFile(aPub); err != nil {
			t.Fatal(err)
		}
		const wantDirect = "D test/b\nD test/dep\n"
		if content, err := os.ReadFile(aPub); err != nil || string(content) != wantDirect {
			t.Fatalf("direct dependency pub = %q, %v; want %q", content, err, wantDirect)
		}
	}
}

func TestGeneratedPubImplicitDependencies(t *testing.T) {
	idx := clang.CreateIndex(1, 1)
	defer idx.Dispose()
	dir := t.TempDir()
	apiHeader := filepath.Join(dir, "api.h")
	libcHeader := filepath.Join(dir, "libc.h")
	runtimeHeader := filepath.Join(dir, "runtime.h")
	libcPub := filepath.Join(dir, "libc.pub")
	runtimePub := filepath.Join(dir, "runtime.pub")
	for name, content := range map[string]string{
		apiHeader:     "#include \"libc.h\"\n#include \"runtime.h\"\nstruct holder { size_t size; runtime_value_t value; };\n",
		libcHeader:    "typedef unsigned long size_t;\n",
		runtimeHeader: "typedef int runtime_value_t;\n",
		libcPub:       "D test/runtime\nT size_t SizeT\n",
		runtimePub:    "T runtime_value_t RuntimeValue\n",
	} {
		if err := os.WriteFile(name, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	libc, err := packages.NewImporter(nil, ".").Import(libcPkgPath)
	if err != nil {
		t.Fatal(err)
	}
	runtimePkg := types.NewPackage("test/runtime", "runtime")
	runtimeType := types.NewTypeName(0, runtimePkg, "RuntimeValue", types.Typ[types.Int32])
	runtimePkg.Scope().Insert(runtimeType)
	runtimePkg.MarkComplete()
	for _, tt := range []struct {
		name     string
		loadLibc bool
		want     string
	}{
		{"implicit_libc_pub", true, "T struct holder Holder\n"},
		{"explicit_libc_pub", false, "D github.com/goplus/lib/c\nD test/runtime\nT struct holder Holder\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pkg := compilePubHeader(t, idx, apiHeader, "test/generated", &Config{
				Importer:        pubTestImporter{libcPkgPath: libc, "test/runtime": runtimePkg},
				LoadLibcPubFile: tt.loadLibc,
				PubFileLookup: func(path string) (string, bool) {
					switch path {
					case libcPkgPath:
						return libcPub, true
					case "test/runtime":
						return runtimePub, true
					}
					return "", false
				},
				PackageOf: func(header string) (string, bool) {
					switch header {
					case apiHeader:
						return "test/generated", true
					case libcHeader:
						return libcPkgPath, true
					case runtimeHeader:
						return "test/runtime", true
					}
					return "", false
				},
			})
			// An implicitly loaded package may appear in the headers again.
			// Its mappings must still resolve, without adding redundant D entries.
			holder := pkg.Types.Scope().Lookup("Holder").Type().Underlying().(*types.Struct)
			if got := holder.Field(1).Type(); got != runtimeType.Type() {
				t.Fatalf("runtime type = %v; want %v", got, runtimeType.Type())
			}
			pub := filepath.Join(t.TempDir(), "llcppg.pub")
			if err := pkg.WritePubFile(pub); err != nil {
				t.Fatal(err)
			}
			if content, err := os.ReadFile(pub); err != nil || string(content) != tt.want {
				t.Fatalf("pub = %q, %v; want %q", content, err, tt.want)
			}
		})
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

func generatePubFixture(t *testing.T, fixtureRoot, workDir, dir string) {
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
	idx := clang.CreateIndex(1, 1)
	defer idx.Dispose()
	u, err := idx.ParseTranslationUnit(clang.DetailedPreprocessingRecord,
		filepath.Join(pkgDir, "test.h"), "-x", "c")
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
		t.Error(goldenDir, ": unexpect result")
	}

	// out.go supplies expected package-level type names only. These tests
	// focus on pub output; comparing Go source bytes would unnecessarily
	// lock its formatting. Import the generated source and check its scope.
	expectedGo, err := parser.ParseFile(token.NewFileSet(), filepath.Join(goldenDir, "out.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := packages.NewImporter(nil, workDir).Import(pkgPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range expectedGo.Decls {
		typesDecl, ok := decl.(*ast.GenDecl)
		if !ok || typesDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range typesDecl.Specs {
			name := spec.(*ast.TypeSpec).Name.Name
			if _, ok := generated.Scope().Lookup(name).(*types.TypeName); !ok {
				t.Errorf("generated package %s is missing expected type %s", pkgPath, name)
			}
		}
	}
}

func TestPubGeneration(t *testing.T) {
	fixtureRoot, workDir := preparePubFixtures(t, "basic")
	generatePubFixture(t, fixtureRoot, workDir, "basic")
}

func TestPubGenerationDependency(t *testing.T) {
	fixtureRoot, workDir := preparePubFixtures(t, "basic", "deps/a")
	generatePubFixture(t, fixtureRoot, workDir, "basic")
	generatePubFixture(t, fixtureRoot, workDir, "deps/a")
}
