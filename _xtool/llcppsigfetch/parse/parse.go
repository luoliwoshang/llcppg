package parse

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/goplus/llcppg/_xtool/llcppsymg/clangutils"
	"github.com/goplus/llcppg/types"
	"github.com/goplus/llgo/c/cjson"
)

type dbgFlags = int

const (
	DbgParse   dbgFlags = 1 << iota
	DbgFlagAll          = DbgParse
)

var (
	debugParse bool
)

func SetDebug(dbgFlags dbgFlags) {
	debugParse = (dbgFlags & DbgParse) != 0
}

type Context struct {
	Files []*FileEntry
	*ContextConfig
}

type ContextConfig struct {
	Conf     *types.Config
	IncFlags []string
}

func NewContext(cfg *ContextConfig) *Context {
	return &Context{
		Files: make([]*FileEntry, 0),
		ContextConfig: &ContextConfig{
			Conf:     cfg.Conf,
			IncFlags: cfg.IncFlags,
		},
	}
}

func (p *Context) Output() *cjson.JSON {
	return MarshalOutputASTFiles(p.Files)
}

// ProcessFiles processes the given files and adds them to the context
func (p *Context) ProcessFiles(files []string) error {
	if debugParse {
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
	if debugParse {
		fmt.Fprintln(os.Stderr, "processFile: path", path)
	}
	for _, entry := range p.Files {
		if entry.Path == path {
			if debugParse {
				fmt.Fprintln(os.Stderr, "processFile: already parsed", path)
			}
			return nil
		}
	}
	parsedFiles, err := p.parseFile(path)
	if err != nil {
		return errors.New("failed to parse file: " + path)
	}

	p.Files = append(p.Files, parsedFiles...)
	fmt.Fprintln(os.Stderr, "processFile:end")
	return nil
}

func (p *Context) parseFile(path string) ([]*FileEntry, error) {
	if debugParse {
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
	// defer converter.Dispose() // 如果单独放开这里的注释，会导致空指针异常错误

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
	fmt.Fprintln(os.Stderr, "parseFile:end")
	return files, nil
}
