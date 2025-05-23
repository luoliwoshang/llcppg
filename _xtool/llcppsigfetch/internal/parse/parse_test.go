package parse_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unsafe"

	"github.com/goplus/lib/c"
	"github.com/goplus/llcppg/_xtool/internal/parser"
	"github.com/goplus/llcppg/_xtool/llcppsigfetch/internal/parse"
	llcppg "github.com/goplus/llcppg/config"
	"github.com/goplus/llpkg/cjson"
)

func TestInclusionMap(t *testing.T) {
	checkFileMap := func(conf *parse.Config, pkg *llcppg.Pkg) {
		found := false
		for path, info := range pkg.FileMap {
			if strings.HasSuffix(path, "sys/types.h") && info.FileType == llcppg.Third {
				found = true
			}
		}
		if !found {
			t.Fatalf("sys/types.h not found")
		}
	}
	err := parse.Do(&parse.Config{
		Conf: &llcppg.Config{
			Include: []string{"sys.h"},
			CFlags:  "-I./testdata/sysinc/hfile",
		},
		Exec: checkFileMap,
	})
	if err != nil {
		panic(err)
	}

}

func TestSystemHeader(t *testing.T) {
	checkFileMap := func(conf *parse.Config, pkg *llcppg.Pkg) {
		for path, info := range pkg.FileMap {
			if path != "testdata/sysinc/hfile/inc.h" && info.FileType != llcppg.Third {
				t.Fatalf("include file is not third header: %s", path)
			}
		}
	}
	err := parse.Do(&parse.Config{
		Conf: &llcppg.Config{
			Include: []string{"inc.h"},
			CFlags:  "-I./testdata/sysinc/hfile",
		},
		Exec: checkFileMap,
	})
	if err != nil {
		panic(err)
	}
}

func TestMacroExpansionOtherFile(t *testing.T) {
	conf := &parse.Config{
		Conf: &llcppg.Config{
			Include: []string{"ref.h"},
			CFlags:  "-I./testdata/macroexpan/hfile",
		},
	}
	testFrom(t, conf, "testdata/macroexpan", false)
}

func testFrom(t *testing.T, conf *parse.Config, dir string, gen bool) {
	pkg := parseWithConfig(conf)

	result := marshalPkg(pkg)
	str := result.Print()
	actual := c.GoString(str)

	expectFile := filepath.Join(dir, "expect.json")
	if gen {
		err := os.WriteFile(expectFile, []byte(actual), os.ModePerm)
		if err != nil {
			t.Fatal("WriteFile failed:", err)
		}
	} else {
		json, err := os.ReadFile(expectFile)
		if err != nil {
			t.Fatal("ReadExpectFile failed:", err)
		}
		expect := string(json)
		if expect != actual {
			t.Fatalf("expect %s, got %s", expect, actual)
		}
	}
	cjson.FreeCStr(unsafe.Pointer(str))
	result.Delete()
}

func parseWithConfig(config *parse.Config) (res *llcppg.Pkg) {
	config.Exec = func(conf *parse.Config, pkg *llcppg.Pkg) {
		res = pkg
	}
	err := parse.Do(config)
	if err != nil {
		panic(err)
	}
	return
}

// for test order map
func marshalPkg(pkg *llcppg.Pkg) *cjson.JSON {
	root := cjson.Object()
	root.SetItem(c.Str("File"), parser.MarshalASTFile(pkg.File))
	root.SetItem(c.Str("FileMap"), marshalFileMap(pkg.FileMap))
	return root
}

// for test order map
func marshalFileMap(fmap map[string]*llcppg.FileInfo) *cjson.JSON {
	root := cjson.Object()
	keys := make([]string, 0, len(fmap))
	for path := range fmap {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	for _, path := range keys {
		root.SetItem(c.AllocaCStr(path), parse.MarshalFileInfo(fmap[path]))
	}
	return root
}
