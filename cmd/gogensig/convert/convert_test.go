package convert_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/goplus/llcppg/_xtool/llcppsymg/args"
	"github.com/goplus/llcppg/ast"
	"github.com/goplus/llcppg/cmd/gogensig/config"
	"github.com/goplus/llcppg/cmd/gogensig/convert"
	"github.com/goplus/llcppg/cmd/gogensig/convert/basic"
	"github.com/goplus/llcppg/cmd/gogensig/dbg"
	"github.com/goplus/llcppg/cmd/gogensig/unmarshal"
	ctoken "github.com/goplus/llcppg/token"
	cppgtypes "github.com/goplus/llcppg/types"
	"github.com/goplus/llgo/xtool/env"
)

func init() {
	dbg.SetDebugAll()
}

func TestFromTestdata(t *testing.T) {
	testFromDir(t, "./_testdata", false)
}

// test sys type in stdinclude to package
func TestSysToPkg(t *testing.T) {
	name := "_systopkg"
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal("Getwd failed:", err)
	}
	testFrom(t, name, path.Join(dir, "_testdata", name), false, func(t *testing.T, testInfo *testInfo) {
		typConv := testInfo.pkg.GetTypeConv()
		if typConv.SysTypeLoc == nil {
			t.Fatal("sysTypeLoc is nil")
		}
		pkgIncTypes := make(map[string]map[string][]string)

		// full type in all std lib
		for name, info := range typConv.SysTypeLoc {
			targetPkg, isDefault := convert.IncPathToPkg(info.IncPath)
			if isDefault {
				targetPkg = "github.com/goplus/llgo/c [default]"
			}
			if pkgIncTypes[targetPkg] == nil {
				pkgIncTypes[targetPkg] = make(map[string][]string, 0)
			}
			if pkgIncTypes[targetPkg][info.IncPath] == nil {
				pkgIncTypes[targetPkg][info.IncPath] = make([]string, 0)
			}
			pkgIncTypes[targetPkg][info.IncPath] = append(pkgIncTypes[targetPkg][info.IncPath], name)
		}

		for pkg, incTypes := range pkgIncTypes {
			t.Logf("\x1b[1;32m %s \x1b[0m Package contains inc types:", pkg)
			for incPath, types := range incTypes {
				t.Logf("\x1b[1;33m  - %s\x1b[0m (%s):", incPath, pkg)
				sort.Strings(types)
				t.Logf("    - %s", strings.Join(types, " "))
			}
		}

		// check referd type in std lib
		// Expected type to package mappings
		expected := map[string]string{
			"mbstate_t":   "github.com/goplus/llgo/c",
			"wint_t":      "github.com/goplus/llgo/c",
			"ptrdiff_t":   "github.com/goplus/llgo/c",
			"int8_t":      "github.com/goplus/llgo/c",
			"max_align_t": "github.com/goplus/llgo/c",
			"FILE":        "github.com/goplus/llgo/c",
			"tm":          "github.com/goplus/llgo/c/time",
			"time_t":      "github.com/goplus/llgo/c/time",
			"clock_t":     "github.com/goplus/llgo/c/time",
			"fenv_t":      "github.com/goplus/llgo/c/math",
			"size_t":      "github.com/goplus/llgo/c",
		}

		for name, exp := range expected {
			if _, ok := typConv.SysTypePkg[name]; ok {
				if typConv.SysTypePkg[name].PkgPath != exp {
					t.Errorf("type [%s]: expected package [%s], got [%s] in header [%s]", name, exp, typConv.SysTypePkg[name].PkgPath, typConv.SysTypePkg[name].Header.IncPath)
				} else {
					t.Logf("refer type [%s] expected package [%s] from header [%s]", name, exp, typConv.SysTypePkg[name].Header.IncPath)
				}
			} else {
				t.Logf("missing expected type %s (package: %s)", name, exp)
			}
		}
	})
}

func TestDepPkg(t *testing.T) {
	name := "_depcjson"
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal("Getwd failed:", err)
	}
	testFrom(t, name, path.Join(dir, "_testdata", name), false, nil)
}

func testFromDir(t *testing.T, relDir string, gen bool) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal("Getwd failed:", err)
	}
	dir = path.Join(dir, relDir)
	fis, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal("ReadDir failed:", err)
	}
	for _, fi := range fis {
		name := fi.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			testFrom(t, name, dir+"/"+name, gen, nil)
		})
	}
}

type testInfo struct {
	pkg       *convert.Package
	fileSet   []*ast.FileEntry
	cfgPath   string
	symbPath  string
	pubPath   string
	expect    string
	outputDir string
}

func testFrom(t *testing.T, name, dir string, gen bool, validateFunc func(t *testing.T, testInfo *testInfo)) {
	confPath := filepath.Join(dir, "conf")
	testInfo := &testInfo{
		symbPath: filepath.Join(confPath, args.LLCPPG_SYMB),
		pubPath:  filepath.Join(confPath, args.LLCPPG_PUB),
		expect:   filepath.Join(dir, "gogensig.expect"),
	}
	var expectContent []byte
	if !gen {
		var err error
		expectContent, err = os.ReadFile(testInfo.expect)
		if err != nil {
			t.Fatal(expectContent)
		}
	}

	cfg, err := config.GetCppgCfgFromPath(filepath.Join(confPath, args.LLCPPG_CFG))
	if err != nil {
		t.Fatal(err)
	}

	// origin cflags + test deps folder cflags,because the test deps 's cflags is depend on machine
	if cfg.CFlags != "" {
		cfg.CFlags = env.ExpandEnv(cfg.CFlags)
	}

	cfg.CFlags += " -I" + filepath.Join(dir, "hfile")
	flagedCfgPath, err := config.CreateTmpJSONFile(args.LLCPPG_CFG, cfg)
	testInfo.cfgPath = flagedCfgPath
	defer os.Remove(flagedCfgPath)

	if err != nil {
		t.Fatal(err)
	}

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = os.Chdir(originalWd)
		if err != nil {
			t.Fatal(err)
		}
	}()
	testInfo.outputDir, err = ModInit(name)
	defer os.RemoveAll(testInfo.outputDir)

	// patch the test file's cflags
	preprocess := func(p *convert.Package) {
		var patchFlags func(pkg *convert.PkgInfo)
		patchFlags = func(pkg *convert.PkgInfo) {
			if pkg.PkgPath != "." {
				incFlags := " -I" + filepath.Join(pkg.Dir, "hfile")
				pkg.CppgConf.CFlags += incFlags
				cfg.CFlags += incFlags
			}

			for _, dep := range pkg.Deps {
				patchFlags(dep)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		patchFlags(p.PkgInfo)
		err = config.CreateJSONFile(flagedCfgPath, cfg)
		if err != nil {
			t.Fatal(err)
		}
	}

	p, pkg, err := basic.ConvertProcesser(&basic.Config{
		PkgPreprocessor: preprocess,
		AstConvertConfig: convert.AstConvertConfig{
			PkgName:   name,
			SymbFile:  testInfo.symbPath,
			CfgFile:   flagedCfgPath,
			OutputDir: testInfo.outputDir,
			PubFile:   testInfo.pubPath,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	bytes, err := config.SigfetchFromConfig(flagedCfgPath, confPath)
	if err != nil {
		t.Fatal(err)
	}

	fileSet, err := unmarshal.FileSet(bytes)
	if err != nil {
		t.Fatal(err)
	}

	err = p.ProcessFileSet(fileSet)
	if err != nil {
		t.Fatal(err)
	}

	testInfo.pkg = pkg
	testInfo.fileSet = fileSet

	var res strings.Builder

	outDir, err := os.ReadDir(testInfo.outputDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, fi := range outDir {
		if strings.HasSuffix(fi.Name(), "go.mod") || strings.HasSuffix(fi.Name(), "go.sum") || strings.HasSuffix(fi.Name(), "llcppg.pub") {
			continue
		} else {
			content, err := os.ReadFile(filepath.Join(testInfo.outputDir, fi.Name()))
			if err != nil {
				t.Fatal(err)
			}
			res.WriteString(fmt.Sprintf("===== %s =====\n", fi.Name()))
			res.Write(content)
			res.WriteString("\n")
		}
	}

	pub, err := os.ReadFile(filepath.Join(testInfo.outputDir, "llcppg.pub"))
	if err == nil {
		res.WriteString("===== llcppg.pub =====\n")
		res.Write(pub)
	}

	if gen {
		if err := os.WriteFile(testInfo.expect, []byte(res.String()), 0644); err != nil {
			t.Fatal(err)
		}
	} else {
		expect := string(expectContent)
		got := res.String()
		if strings.TrimSpace(expect) != strings.TrimSpace(got) {
			t.Errorf("does not match expected.\nExpected:\n%s\nGot:\n%s", expect, got)
		}
	}

	if validateFunc != nil {
		validateFunc(t, testInfo)
	}
}

// ===========================error
func TestNewAstConvert(t *testing.T) {
	_, err := convert.NewAstConvert(&convert.AstConvertConfig{
		PkgName:  "test",
		SymbFile: "",
		CfgFile:  "",
	})
	if err != nil {
		t.Fatal("NewAstConvert Fail")
	}
}

func TestNewAstConvertFail(t *testing.T) {
	_, err := convert.NewAstConvert(nil)
	if err == nil {
		t.Fatal("no error")
	}
}

func TestVisitDone(t *testing.T) {
	pkg, err := convert.NewAstConvert(&convert.AstConvertConfig{
		PkgName:  "test",
		SymbFile: "",
		CfgFile:  "",
	})
	if err != nil {
		t.Fatal("NewAstConvert Fail")
	}
	pkg.SetVisitDone(func(pkg *convert.Package, incPath string) {
		if incPath != "test.h" {
			t.Fatal("doc path error")
		}
	})
	pkg.VisitDone("test.h")
}

func TestVisitFail(t *testing.T) {
	converter, err := convert.NewAstConvert(&convert.AstConvertConfig{
		PkgName:  "test",
		SymbFile: "",
		CfgFile:  "",
	})
	if err != nil {
		t.Fatal("NewAstConvert Fail")
	}

	// expect type
	converter.VisitTypedefDecl(&ast.TypedefDecl{
		Name: &ast.Ident{Name: "NormalType"},
		Type: &ast.BuiltinType{Kind: ast.Int},
	})

	// not appear in output,because expect error
	converter.VisitTypedefDecl(&ast.TypedefDecl{
		Name: &ast.Ident{Name: "Foo"},
		Type: nil,
	})

	errRecordType := &ast.RecordType{
		Tag: ast.Struct,
		Fields: &ast.FieldList{
			List: []*ast.Field{
				{Type: &ast.BuiltinType{Kind: ast.Int, Flags: ast.Double}},
			},
		},
	}
	// error field type for struct
	converter.VisitStruct(&ast.Ident{Name: "Foo"}, nil, &ast.TypeDecl{
		Name: &ast.Ident{Name: "Foo"},
		Type: errRecordType,
	})

	// error field type for anonymous struct
	converter.VisitStruct(&ast.Ident{Name: "Foo"}, nil, &ast.TypeDecl{
		Name: nil,
		Type: errRecordType,
	})

	converter.VisitStruct(&ast.Ident{Name: "Union (unnamed at /usr/local/Cellar/msgpack/6.0.2/include/msgpack/object.h:75:9)"}, nil, &ast.TypeDecl{
		Name: &ast.Ident{Name: "Union (unnamed at /usr/local/Cellar/msgpack/6.0.2/include/msgpack/object.h:75:9)"},
		Type: errRecordType,
	})

	converter.VisitEnumTypeDecl(&ast.EnumTypeDecl{
		Name: &ast.Ident{Name: "NormalType"},
		Type: &ast.EnumType{},
	})

	// error enum item for anonymous enum
	converter.VisitEnumTypeDecl(&ast.EnumTypeDecl{
		Name: nil,
		Type: &ast.EnumType{
			Items: []*ast.EnumItem{
				{Name: &ast.Ident{Name: "Item1"}},
			},
		},
	})

	converter.VisitFuncDecl(&ast.FuncDecl{
		Name: &ast.Ident{Name: "Foo"},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{Type: &ast.BuiltinType{Kind: ast.Int, Flags: ast.Double}},
				},
			},
		},
	})

	converter.VisitMacro(&ast.Macro{
		Name: "Foo",
		Tokens: []*ast.Token{
			{Token: ctoken.IDENT, Lit: "Foo"},
			{Token: ctoken.LITERAL, Lit: "1"},
		},
	})
	// not appear in output

	buf, err := converter.Pkg.WriteDefaultFileToBuffer()
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	expectedOutput :=
		`
package test

import (
	"github.com/goplus/llgo/c"
	_ "unsafe"
)

type NormalType c.Int
type Foo struct {
	Unused [8]uint8
}
`
	if strings.TrimSpace(expectedOutput) != strings.TrimSpace(buf.String()) {
		t.Errorf("does not match expected.\nExpected:\n%s\nGot:\n%s", expectedOutput, buf.String())
	}
}

func TestWritePkgFilesFail(t *testing.T) {
	tempDir, err := os.MkdirTemp(dir, "test_package_write_unwritable")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	converter, err := convert.NewAstConvert(&convert.AstConvertConfig{
		PkgName:   "test",
		SymbFile:  "",
		CfgFile:   "",
		OutputDir: tempDir,
	})
	if err != nil {
		t.Fatal("NewAstConvert Fail")
	}
	err = os.Chmod(tempDir, 0555)
	defer func() {
		if err := os.Chmod(tempDir, 0755); err != nil {
			t.Fatalf("Failed to change directory permissions: %v", err)
		}
	}()
	if err != nil {
		t.Fatalf("Failed to change directory permissions: %v", err)
	}
	converter.VisitStart("test.h", "/path/to/test.h", false)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic, but got: %v", r)
		}
	}()
	converter.WritePkgFiles()
}

func TestGetIncPathFail(t *testing.T) {
	cfg, err := config.CreateTmpJSONFile("llcppg.cfg", &cppgtypes.Config{
		Include: []string{"unexist.h"},
	})
	if err != nil {
		t.Fatal(err)
	}
	converter, err := convert.NewAstConvert(&convert.AstConvertConfig{
		PkgName:  "test",
		SymbFile: "",
		CfgFile:  cfg,
	})
	if err != nil {
		t.Fatal("NewAstConvert Fail")
	}
	converter.VisitStart("test.h", "", false)
}

func ModInit(name string) (string, error) {
	tempDir, err := os.MkdirTemp("", "gogensig-test")
	if err != nil {
		return "", err
	}
	outputDir := filepath.Join(tempDir, name)
	err = os.MkdirAll(outputDir, 0744)
	if err != nil {
		return "", err
	}
	projectRoot, err := filepath.Abs("../../../")
	if err != nil {
		return "", err
	}
	if err := os.Chdir(outputDir); err != nil {
		return "", err
	}

	err = config.RunCommand(outputDir, "go", "mod", "init", name)
	if err != nil {
		return "", err
	}
	err = config.RunCommand(outputDir, "go", "get", "github.com/goplus/llgo@main")
	if err != nil {
		return "", err
	}
	err = config.RunCommand(outputDir, "go", "get", "github.com/goplus/llcppg")
	if err != nil {
		return "", err
	}
	err = config.RunCommand(outputDir, "go", "mod", "edit", "-replace", "github.com/goplus/llcppg="+projectRoot)
	if err != nil {
		return "", err
	}
	return outputDir, nil
}

type convertTestInfo struct {
	testInfo *testInfo
	cvt      *convert.Converter
}

func testFromConvert(t *testing.T, name, dir string, gen bool, validateFunc func(t *testing.T, testInfo *convertTestInfo)) {
	confPath := filepath.Join(dir, "conf")
	testInfo := &testInfo{
		symbPath: filepath.Join(confPath, args.LLCPPG_SYMB),
		pubPath:  filepath.Join(confPath, args.LLCPPG_PUB),
		expect:   filepath.Join(dir, "gogensig.expect"),
	}
	var expectContent []byte
	if !gen {
		var err error
		expectContent, err = os.ReadFile(testInfo.expect)
		if err != nil {
			t.Fatal(expectContent)
		}
	}

	cfg, err := config.GetCppgCfgFromPath(filepath.Join(confPath, args.LLCPPG_CFG))
	if err != nil {
		t.Fatal(err)
	}

	// origin cflags + test deps folder cflags,because the test deps 's cflags is depend on machine
	if cfg.CFlags != "" {
		cfg.CFlags = env.ExpandEnv(cfg.CFlags)
	}

	cfg.CFlags += " -I" + filepath.Join(dir, "hfile")
	flagedCfgPath, err := config.CreateTmpJSONFile(args.LLCPPG_CFG, cfg)
	testInfo.cfgPath = flagedCfgPath
	defer os.Remove(flagedCfgPath)

	if err != nil {
		t.Fatal(err)
	}

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Fatal(err)
		}
	}()
	testInfo.outputDir, err = ModInit(name)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(testInfo.outputDir)

	cvt, err := convert.NewConverter(&convert.ConverterConfig{
		PkgName:   name,
		SymbFile:  testInfo.symbPath,
		CfgFile:   flagedCfgPath,
		OutputDir: testInfo.outputDir,
		PubFile:   testInfo.pubPath,
	})
	testInfo.pkg = cvt.Pkg

	if err != nil {
		t.Fatal(err)
	}

	bytes, err := config.SigfetchFromConfig(flagedCfgPath, confPath)
	if err != nil {
		t.Fatal(err)
	}

	fileSet, err := unmarshal.FileSet(bytes)
	if err != nil {
		t.Fatal(err)
	}
	testInfo.fileSet = fileSet

	cvt.Collect(fileSet)
	err = cvt.Start()
	if err != nil {
		t.Fatal(err)
	}

	var res strings.Builder

	outDir, err := os.ReadDir(testInfo.outputDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, fi := range outDir {
		if strings.HasSuffix(fi.Name(), "go.mod") || strings.HasSuffix(fi.Name(), "go.sum") || strings.HasSuffix(fi.Name(), "llcppg.pub") {
			continue
		} else {
			var content []byte
			content, err = os.ReadFile(filepath.Join(testInfo.outputDir, fi.Name()))
			if err != nil {
				t.Fatal(err)
			}
			res.WriteString(fmt.Sprintf("===== %s =====\n", fi.Name()))
			res.Write(content)
			res.WriteString("\n")
		}
	}

	pub, err := os.ReadFile(filepath.Join(testInfo.outputDir, "llcppg.pub"))
	if err == nil {
		res.WriteString("===== llcppg.pub =====\n")
		res.Write(pub)
	}

	if gen {
		if err := os.WriteFile(testInfo.expect, []byte(res.String()), 0644); err != nil {
			t.Fatal(err)
		}
	} else {
		expect := string(expectContent)
		got := res.String()
		if strings.TrimSpace(expect) != strings.TrimSpace(got) {
			t.Errorf("does not match expected.\nExpected:\n%s\nGot:\n%s", expect, got)
		}
	}

	if validateFunc != nil {
		validateFunc(t, &convertTestInfo{
			testInfo: testInfo,
			cvt:      cvt,
		})
	}
}

func TestConvertTypedef(t *testing.T) {
	name := "typedef"
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal("Getwd failed:", err)
	}
	testFromConvert(t, name, filepath.Join(dir, "_testdata", name), false, func(t *testing.T, testInfo *convertTestInfo) {
		cvt := testInfo.cvt
		fileSet := cvt.FileSet
		usrOrder, err := testInfo.cvt.BuildOrder()
		if err != nil {
			t.Fatal(err)
		}
		decls := fileSet[0].Doc.Decls
		// typedef a,b,c,d,e,f
		typedefs := []ast.Decl{
			decls[1],  // typedef a
			decls[3],  // typedef b
			decls[5],  // typedef c
			decls[7],  // typedef d
			decls[9],  // typedef e
			decls[11], // typedef f
		}
		for _, decl := range typedefs {
			if typedef, ok := decl.(*ast.TypedefDecl); ok {
				tagExpr, ok := typedef.Type.(*ast.TagExpr)
				if !ok {
					t.Fatalf("Expect *ast.TagExpr")
				}
				typedefIdent, ok := tagExpr.Name.(*ast.Ident)
				if !ok {
					t.Fatalf("Expect *ast.Ident")
				}
				underlying, ok := cvt.Types.Lookup(typedefIdent.USR)
				if !ok {
					t.Fatalf("Expect %s, but not found", typedefIdent.Name)
				}
				_, isTypedef := underlying.(*ast.TypedefDecl)
				if isTypedef {
					t.Fatalf("Expect %s, but found TypedefDecl", typedefIdent.Name)
				}
				var underlyingName *ast.Ident
				switch typ := underlying.(type) {
				case *ast.TypeDecl:
					underlyingName = underlying.(*ast.TypeDecl).Name
				case *ast.EnumTypeDecl:
					underlyingName = underlying.(*ast.EnumTypeDecl).Name
				default:
					t.Fatalf("Found Unexpected Underlying Type %T", typ)
				}
				if underlyingName.Name != typedefIdent.Name {
					t.Fatalf("Underlying Name Expect %s, but found %s", typedefIdent.Name, underlyingName.Name)
				}
				if underlyingName.USR != typedefIdent.USR {
					t.Fatalf("Underlying Name USR Expect %s, but found %s", typedefIdent.USR, underlyingName.USR)
				}

			}
		}

		for _, typedef := range typedefs {
			typedefDecl := typedef.(*ast.TypedefDecl)
			deps := cvt.Types.LookupDeps(typedefDecl.Name.USR)
			if len(deps) != 1 {
				t.Fatalf("Expect 1 dep, but found %d", len(deps))
			}
			depAnonyType, ok := cvt.Types.Lookup(deps[0])
			if !ok {
				t.Fatalf("Expect %s, but not found", deps[0])
			}
			switch anonType := depAnonyType.(type) {
			case *ast.TypeDecl:
				if anonType.Name.Name != typedefDecl.Name.Name {
					t.Fatalf("Expect %s, but found %s", typedefDecl.Name.Name, anonType.Name.Name)
				}
			case *ast.EnumTypeDecl:
				if anonType.Name.Name != typedefDecl.Name.Name {
					t.Fatalf("Expect %s, but found %s", typedefDecl.Name.Name, anonType.Name.Name)
				}
			}
		}

		if len(usrOrder) != len(cvt.Types.Defs) {
			t.Fatalf("Expect USR Order %d, but found %d", len(cvt.Types.Defs), len(usrOrder))
		}
	})
}

func TestAvoidKeyword(t *testing.T) {
	name := "avoidkeyword"
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal("Getwd failed:", err)
	}
	testFromConvert(t, name, filepath.Join(dir, "_testdata", name), false, nil)
}

func TestKeepComment(t *testing.T) {
	name := "keepcomment"
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal("Getwd failed:", err)
	}
	testFromConvert(t, name, filepath.Join(dir, "_testdata", name), false, nil)
}

func TestNested(t *testing.T) {
	name := "nested"
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal("Getwd failed:", err)
	}
	testFromConvert(t, name, filepath.Join(dir, "_testdata", name), false, nil)
}

// [NIT] CustomData struct have a addtional line
// func TestPubfile(t *testing.T) {
// 	name := "pubfile"
// 	dir, err := os.Getwd()
// 	if err != nil {
// 		t.Fatal("Getwd failed:", err)
// 	}
// 	testFromConvert(t, name, filepath.Join(dir, "_testdata", name), false, nil)
// }

// Expect diffrent
// func TestReceiver(t *testing.T) {
// 	name := "receiver"
// 	dir, err := os.Getwd()
// 	if err != nil {
// 		t.Fatal("Getwd failed:", err)
// 	}
// 	testFromConvert(t, name, filepath.Join(dir, "_testdata", name), false, nil)
// }

func TestSelfRef(t *testing.T) {
	name := "selfref"
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal("Getwd failed:", err)
	}
	testFromConvert(t, name, filepath.Join(dir, "_testdata", name), true, nil)
}
