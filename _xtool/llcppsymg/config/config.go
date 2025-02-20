package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/goplus/llcppg/_xtool/llcppsymg/clangutils"
	"github.com/goplus/llcppg/llcppg"
	"github.com/goplus/llgo/c"
	"github.com/goplus/llgo/c/cjson"
	"github.com/goplus/llgo/c/clang"
)

type Conf struct {
	*cjson.JSON
	*llcppg.Config
}

func GetConf(data []byte) (Conf, error) {
	parsedConf := cjson.ParseBytes(data)
	if parsedConf == nil {
		return Conf{}, errors.New("failed to parse config")
	}

	config := &llcppg.Config{
		Name:         GetStringItem(parsedConf, "name", ""),
		CFlags:       GetStringItem(parsedConf, "cflags", ""),
		Libs:         GetStringItem(parsedConf, "libs", ""),
		Include:      GetStringArrayItem(parsedConf, "include"),
		TrimPrefixes: GetStringArrayItem(parsedConf, "trimPrefixes"),
		Cplusplus:    GetBoolItem(parsedConf, "cplusplus"),
	}

	return Conf{
		JSON:   parsedConf,
		Config: config,
	}, nil
}

func GetString(obj *cjson.JSON) (value string) {
	str := obj.GetStringValue()
	return unsafe.String((*byte)(unsafe.Pointer(str)), c.Strlen(str))
}

func GetStringItem(obj *cjson.JSON, key string, defval string) (value string) {
	item := obj.GetObjectItemCaseSensitive(c.AllocaCStr(key))
	if item == nil {
		return defval
	}
	return GetString(item)
}

func GetStringArrayItem(obj *cjson.JSON, key string) (value []string) {
	item := obj.GetObjectItemCaseSensitive(c.AllocaCStr(key))
	if item == nil {
		return
	}
	value = make([]string, item.GetArraySize())
	for i := range value {
		value[i] = GetString(item.GetArrayItem(c.Int(i)))
	}
	return
}

func GetBoolItem(obj *cjson.JSON, key string) bool {
	item := obj.GetObjectItemCaseSensitive(c.AllocaCStr(key))
	if item == nil {
		return false
	}
	if item.IsBool() != 0 {
		return item.IsTrue() != 0
	}
	return false
}

type PkgHfilesInfo struct {
	Inters map[string]struct{} // From types.Config.Include
	Impls  map[string]struct{} // From same root of types.Config.Include
	Thirds map[string]struct{} // Not Current Pkg's Files
}

func (p *PkgHfilesInfo) CurPkgFiles() []string {
	curPkgFiles := []string{}
	for f := range p.Inters {
		curPkgFiles = append(curPkgFiles, f)
	}
	for f := range p.Impls {
		curPkgFiles = append(curPkgFiles, f)
	}
	return curPkgFiles
}

// PkgHfileInfo analyzes header files dependencies and categorizes them into three groups:
// 1. Inters: Direct includes from types.Config.Include
// 2. Impls: Header files from the same root directory as Inters
// 3. Thirds: Header files from external sources
//
// The function works by:
// 1. Creating a temporary header file that includes all headers from conf.Include
// 2. Using clang to parse the translation unit and analyze includes
// 3. Categorizing includes based on their inclusion level and path relationship
func PkgHfileInfo(conf *llcppg.Config, args []string) *PkgHfilesInfo {
	info := &PkgHfilesInfo{
		Inters: make(map[string]struct{}),
		Impls:  make(map[string]struct{}),
		Thirds: make(map[string]struct{}),
	}
	outfile, err := os.CreateTemp("", "compose_*.h")
	if err != nil {
		panic(err)
	}

	cflags := append(args, strings.Fields(conf.CFlags)...)
	clangutils.ComposeIncludes(conf.Include, outfile.Name())
	index, unit, err := clangutils.CreateTranslationUnit(&clangutils.Config{
		File: outfile.Name(),
		Temp: false,
		Args: cflags,
	})
	if err != nil {
		panic(err)
	}
	defer unit.Dispose()
	defer index.Dispose()

	inters := []string{}
	others := []string{} // impl & third
	clangutils.GetInclusions(unit, func(inced clang.File, incins []clang.SourceLocation) {
		// first level include is the conf.include's abs path
		filename := clang.GoString(inced.FileName())
		if len(incins) == 1 {
			inters = append(inters, filename)
			info.Inters[filename] = struct{}{}
		} else {
			// not in the first level include maybe impl or third hfile
			_, inter := info.Inters[filename]
			if len(incins) > 1 && !inter {
				others = append(others, filename)
			}
		}
	})

	if conf.Mix {
		for _, f := range others {
			info.Thirds[f] = struct{}{}
		}
		return info
	}

	root, err := filepath.Abs(commonParentDir(inters))
	if err != nil {
		panic(err)
	}
	for _, f := range others {
		file, err := filepath.Abs(f)
		if err != nil {
			panic(err)
		}
		if strings.HasPrefix(file, root) {
			info.Impls[f] = struct{}{}
		} else {
			info.Thirds[f] = struct{}{}
		}
	}
	return info
}

// commonParentDir finds the longest common parent directory path for a given slice of paths.
// For example, given paths ["/a/b/c/d", "/a/b/e/f"], it returns "/a/b".
func commonParentDir(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	parts := make([][]string, len(paths))
	for i, path := range paths {
		parts[i] = strings.Split(filepath.Dir(path), string(filepath.Separator))
	}

	for i := 0; i < len(parts[0]); i++ {
		for j := 1; j < len(parts); j++ {
			if i == len(parts[j]) || parts[j][i] != parts[0][i] {
				return filepath.Join(parts[0][:i]...)
			}
		}
	}
	return filepath.Dir(paths[0])
}
