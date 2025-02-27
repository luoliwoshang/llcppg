package parse

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/goplus/llcppg/_xtool/llcppsigfetch/dbg"
	"github.com/goplus/llcppg/_xtool/llcppsymg/clangutils"
	"github.com/goplus/llcppg/_xtool/llcppsymg/config"
	"github.com/goplus/llcppg/llcppg"
	"github.com/goplus/llgo/c/cjson"
)

type Context struct {
	FileSet []*llcppg.FileEntry
	*ContextConfig
}

type ContextConfig struct {
	Conf        *llcppg.Config
	IncFlags    []string
	PkgFileInfo *config.PkgHfilesInfo
}

func NewContext(cfg *ContextConfig) *Context {
	return &Context{
		FileSet:       make([]*llcppg.FileEntry, 0),
		ContextConfig: cfg,
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

func (p *Context) parseFile(path string) ([]*llcppg.FileEntry, error) {
	if dbg.GetDebugParse() {
		fmt.Fprintln(os.Stderr, "parseFile: path", path)
		fmt.Fprintln(os.Stderr, p.PkgFileInfo != nil)
	}
	converter, err := NewConverter(&clangutils.Config{
		File:  path,
		Temp:  false,
		IsCpp: p.Conf.Cplusplus,
		Args:  strings.Fields(p.Conf.CFlags),
	}, p.PkgFileInfo)
	if err != nil {
		return nil, errors.New("failed to create converter " + path)
	}
	defer converter.Dispose()

	files, err := converter.Convert()

	if err != nil {
		return nil, err
	}

	return files, nil
}

func Do(conf *llcppg.Config) (*Context, error) {
	pkgHfiles := config.PkgHfileInfo(conf, []string{})
	if dbg.GetDebugParse() {
		fmt.Fprintln(os.Stderr, "interfaces", pkgHfiles.Inters)
		fmt.Fprintln(os.Stderr, "implements", pkgHfiles.Impls)
		fmt.Fprintln(os.Stderr, "thirdhfile", pkgHfiles.Thirds)
	}

	context := NewContext(&ContextConfig{
		Conf:        conf,
		PkgFileInfo: pkgHfiles,
	})
	err := context.ProcessFiles(pkgHfiles.Inters)
	if err != nil {
		return nil, err
	}
	return context, nil
}
