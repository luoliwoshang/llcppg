package parse

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/goplus/llcppg/_xtool/llcppsigfetch/dbg"
	"github.com/goplus/llcppg/_xtool/llcppsymg/clangutils"
	"github.com/goplus/llcppg/ast"
	"github.com/goplus/llcppg/types"
	"github.com/goplus/llgo/c/cjson"
)

// temp to avoid call clang in llcppsigfetch,will cause hang
var ClangSearchPath []string
var ClangResourceInclude string

type Context struct {
	FileSet []*ast.FileEntry
	*ContextConfig
}

type ContextConfig struct {
	Conf     *types.Config
	IncFlags []string
}

func NewContext(cfg *ContextConfig) *Context {
	return &Context{
		FileSet: make([]*ast.FileEntry, 0),
		ContextConfig: &ContextConfig{
			Conf:     cfg.Conf,
			IncFlags: cfg.IncFlags,
		},
	}
}

func (p *Context) Output() *cjson.JSON {
	return MarshalFileSet(p.FileSet)
}

// ProcessFiles processes the given files and adds them to the context
func (p *Context) ProcessFiles(files []string) error {
	if dbg.GetDebugParse() {
		fmt.Fprintln(os.Stderr, "ProcessFiles: files", files, "isCpp", p.Conf.Cplusplus)
	}
	for _, file := range files {
		if err := p.processFile(file); err != nil {
			return err
		}
	}
	return nil
}

// parse file and add it to the context,avoid duplicate parsing
func (p *Context) processFile(path string) error {
	if dbg.GetDebugParse() {
		fmt.Fprintln(os.Stderr, "processFile: path", path)
	}
	for _, entry := range p.FileSet {
		if entry.Path == path {
			if dbg.GetDebugParse() {
				fmt.Fprintln(os.Stderr, "processFile: already parsed", path)
			}
			return nil
		}
	}
	parsedFiles, err := p.parseFile(path)
	if err != nil {
		return errors.New("failed to parse file: " + path)
	}

	p.FileSet = append(p.FileSet, parsedFiles...)
	return nil
}

func (p *Context) parseFile(path string) ([]*ast.FileEntry, error) {
	if dbg.GetDebugParse() {
		fmt.Fprintln(os.Stderr, "parseFile: path", path)
	}
	converter, err := NewConverter(&clangutils.Config{
		File:  path,
		Temp:  false,
		IsCpp: p.Conf.Cplusplus,
		Args:  p.IncFlags,
	})
	if err != nil {
		return nil, errors.New("failed to create converter " + path)
	}
	defer converter.Dispose()

	files, err := converter.Convert()

	// the entry file is the first file in the files list
	entryFile := files[0]
	if entryFile.IncPath != "" {
		return nil, errors.New("entry file " + entryFile.Path + " has include path " + entryFile.IncPath)
	}

	for _, include := range p.Conf.Include {
		if strings.Contains(entryFile.Path, include) {
			entryFile.IncPath = include
			break
		}
	}

	if entryFile.IncPath == "" {
		return nil, errors.New("entry file " + entryFile.Path + " is not in include list")
	}

	if err != nil {
		return nil, err
	}

	return files, nil
}

type ParseConfig struct {
	Conf             *types.Config
	CombinedFile     string
	PreprocessedFile string
	OutputFile       bool
}

func Do(cfg *ParseConfig) (*types.Pkg, error) {
	if cfg.CombinedFile == "" {
		combinedFile, err := os.CreateTemp("", cfg.Conf.Name+"*.h")
		if err != nil {
			return nil, err
		}
		defer combinedFile.Close()
		cfg.CombinedFile = combinedFile.Name()
	}

	if cfg.PreprocessedFile == "" {
		preprocessedFile, err := os.CreateTemp("", cfg.Conf.Name+"*.i")
		if err != nil {
			return nil, err
		}
		defer preprocessedFile.Close()
		cfg.PreprocessedFile = preprocessedFile.Name()
	}

	if dbg.GetDebugParse() {
		fmt.Fprintln(os.Stderr, "Do: combinedFile", cfg.CombinedFile)
		fmt.Fprintln(os.Stderr, "Do: preprocessedFile", cfg.PreprocessedFile)
	}
	err := clangutils.ComposeIncludes(cfg.Conf.Include, cfg.CombinedFile)
	if err != nil {
		return nil, err
	}

	clangFlags := strings.Fields(cfg.Conf.CFlags)
	// clangFlags = append(clangFlags, "-nobuiltininc")
	// to avoid libclang & clang different search path,but it will cause
	// 	   /opt/homebrew/include/lua/lua.h:11:10: fatal error: 'stdarg.h' file not found
	//     11 | #include <stdarg.h>
	// so use llvm-config --cflags to libclang to ensure the same search path for libclang and clang
	clangFlags = append(clangFlags, "-C")  // keep comment
	clangFlags = append(clangFlags, "-dD") // keep macro

	err = clangutils.Preprocess(&clangutils.PreprocessConfig{
		File:    cfg.CombinedFile,
		IsCpp:   cfg.Conf.Cplusplus,
		Args:    clangFlags,
		OutFile: cfg.PreprocessedFile,
	})
	if err != nil {
		return nil, err
	}
	libclangFlags := []string{}
	if len(ClangSearchPath) != 0 {
		for _, path := range ClangSearchPath {
			libclangFlags = append(libclangFlags, "-I"+path)
		}
	}
	libclangFlags = append(libclangFlags, strings.Fields(cfg.Conf.CFlags)...)
	// llvm cflags is not clang's include search path
	converter, err := NewConverterX(
		&Config{
			CombinedFile: cfg.CombinedFile,
			Cfg: &clangutils.Config{
				File:  cfg.PreprocessedFile,
				IsCpp: cfg.Conf.Cplusplus,
				Args:  libclangFlags,
			},
		})
	if err != nil {
		return nil, err
	}
	pkg, err := converter.ConvertX()
	if err != nil {
		return nil, err
	}

	return pkg, nil
}

func llvmCflags() []string {
	out, err := exec.Command("llvm-config", "--cflags").Output()
	if err != nil {
		panic(err)
	}
	return strings.Fields(string(out))
}
