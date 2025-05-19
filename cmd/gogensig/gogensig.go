/*
 * Copyright (c) 2024 The GoPlus Authors (goplus.org). All rights reserved.
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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/gogen"
	args "github.com/goplus/llcppg/_xtool/llcppsymg/tool/arg"
	name "github.com/goplus/llcppg/_xtool/llcppsymg/tool/name"
	"github.com/goplus/llcppg/ast"
	"github.com/goplus/llcppg/cl"
	"github.com/goplus/llcppg/cmd/gogensig/config"
	"github.com/goplus/llcppg/cmd/gogensig/unmarshal"
	llcppg "github.com/goplus/llcppg/config"
	"github.com/qiniu/x/errors"
)

func main() {
	ags, remainArgs := args.ParseArgs(os.Args[1:], "-", nil)

	if ags.Help {
		printUsage()
		return
	}

	if ags.Verbose {
		cl.SetDebug(cl.DbgFlagAll)
	}

	var cfgFile string
	var modulePath string
	for i := 0; i < len(remainArgs); i++ {
		arg := remainArgs[i]
		if strings.HasPrefix(arg, "-cfg=") {
			cfgFile = args.StringArg(arg, llcppg.LLCPPG_CFG)
		}
		if strings.HasPrefix(arg, "-mod=") {
			modulePath = args.StringArg(arg, "")
		}
	}
	if cfgFile == "" {
		cfgFile = llcppg.LLCPPG_CFG
	}

	conf, err := config.GetCppgCfgFromPath(cfgFile)
	check(err)
	wd, err := os.Getwd()
	check(err)

	outputDir := filepath.Join(wd, conf.Name)

	err = prepareEnv(outputDir, conf.Deps, modulePath)
	check(err)

	data, err := config.ReadSigfetchFile(filepath.Join(wd, ags.CfgFile))
	check(err)

	convertPkg, err := unmarshal.Pkg(data)
	check(err)

	symbFile := filepath.Join(wd, llcppg.LLCPPG_SYMB)
	symbTable, err := config.NewSymbolTable(symbFile)
	check(err)

	pkg, err := cl.Convert(&cl.ConvConfig{
		PkgName: conf.Name,
		ConvSym: func(name *ast.Object, mangleName string) (goName string, err error) {
			item, err := symbTable.LookupSymbol(mangleName)
			if err != nil {
				return
			}
			return item.GoName, nil
		},
		NodeConv: NewNodeConverter(
			&NodeConverterConfig{
				PkgName:      conf.Name,
				SymbTable:    symbTable,
				FileMap:      convertPkg.FileMap,
				TypeMap:      conf.TypeMap,
				TrimPrefixes: conf.TrimPrefixes,
			},
		),
		Pkg:            convertPkg.File,
		FileMap:        convertPkg.FileMap,
		TypeMap:        conf.TypeMap,
		Deps:           conf.Deps,
		TrimPrefixes:   conf.TrimPrefixes,
		Libs:           conf.Libs,
		KeepUnderScore: conf.KeepUnderScore,
	})
	check(err)

	err = config.WritePubFile(filepath.Join(outputDir, llcppg.LLCPPG_PUB), pkg.Pubs)
	check(err)

	err = writePkg(pkg.Package, outputDir)
	check(err)

	err = config.RunCommand(outputDir, "go", "fmt", ".")
	check(err)

	err = config.RunCommand(outputDir, "go", "mod", "tidy")
	check(err)
}

// todo(zzy):move out in gogensig.go
type NodeConverter struct {
	symbols *ProcessSymbol
	conf    *NodeConverterConfig
}

type NodeConverterConfig struct {
	PkgName      string
	SymbTable    *config.SymbolTable
	FileMap      map[string]*llcppg.FileInfo
	TrimPrefixes []string
	TypeMap      map[string]string
}

func NewNodeConverter(cfg *NodeConverterConfig) *NodeConverter {
	return &NodeConverter{
		symbols: NewProcessSymbol(),
		conf:    cfg,
	}
}

func (p *NodeConverter) ConvDecl(decl ast.Decl) (goName, goFile string, err error) {
	switch decl := decl.(type) {
	case *ast.FuncDecl:
		goFile, err = p.goFile(decl.Loc.File)
		if err != nil {
			return
		}
		var item *config.SymbolEntry
		item, err = p.conf.SymbTable.LookupSymbol(decl.MangledName)
		if err != nil {
			return
		}
		return item.GoName, "", nil
	case *ast.TypeDecl:
		goFile, err = p.goFile(decl.Loc.File)
		if err != nil {
			return
		}
		goName, _ := p.GetUniqueName(Node{name: decl.Name.Name, kind: TypeDecl}, p.declName)
		return goName, goFile, nil
	case *ast.TypedefDecl:
		goFile, err = p.goFile(decl.Loc.File)
		if err != nil {
			return
		}
		goName, _ := p.GetUniqueName(Node{name: decl.Name.Name, kind: TypedefDecl}, p.declName)
		return goName, goFile, nil
	case *ast.EnumTypeDecl:
		goFile, err = p.goFile(decl.Loc.File)
		if err != nil {
			return
		}
		goName, _ := p.GetUniqueName(Node{name: decl.Name.Name, kind: EnumTypeDecl}, p.declName)
		return goName, goFile, nil
	}
	return "", "", fmt.Errorf("unsupported decl type: %T", decl)
}

func (p *NodeConverter) ConvEnumItem(decl *ast.EnumTypeDecl, item *ast.EnumItem) (goName, goFile string, err error) {
	goFile, err = p.goFile(decl.Loc.File)
	if err != nil {
		return
	}
	goName, _ = p.GetUniqueName(Node{name: item.Name.Name, kind: EnumItem}, p.constName)
	return goName, goFile, nil
}

func (p *NodeConverter) ConvMacro(macro *ast.Macro) (goName, goFile string, err error) {
	goFile, err = p.goFile(macro.Loc.File)
	if err != nil {
		return
	}
	goName, _ = p.GetUniqueName(Node{name: macro.Name, kind: Macro}, p.constName)
	return goName, goFile, nil
}

type NameMethod func(name string) string

func (p *NodeConverter) goFile(file string) (string, error) {
	info, ok := p.conf.FileMap[file]
	if !ok {
		var availableFiles []string
		for f := range p.conf.FileMap {
			availableFiles = append(availableFiles, f)
		}
		return "", fmt.Errorf("file %q not found in FileMap. Available files:\n%s",
			file, strings.Join(availableFiles, "\n"))
	}
	switch info.FileType {
	case llcppg.Inter:
		return name.HeaderFileToGo(file), nil
	case llcppg.Impl:
		return p.conf.PkgName + "_autogen.go", nil
	default:
		return "", cl.ErrSkip
	}
}

// GetUniqueName generates a unique public name for a given node using the provided name transformation method.
// It ensures the generated name doesn't conflict with existing names by adding a numeric suffix if needed.
//
// Parameters:
//   - node: The node containing the original name to be transformed
//   - nameMethod: Function used to transform the original name (e.g., declName, constName)
//
// Returns:
//   - pubName: The generated unique public name
//   - changed: Whether the generated name differs from the original name
func (p *NodeConverter) GetUniqueName(node Node, nameMethod NameMethod) (pubName string, changed bool) {
	pubName = nameMethod(node.name)
	uniquePubName := p.symbols.Register(node, pubName)
	return uniquePubName, uniquePubName != node.name
}

// which is define in llcppg.cfg/typeMap
func (p *NodeConverter) definedName(name string) (string, bool) {
	definedName, ok := p.conf.TypeMap[name]
	if ok {
		if definedName == "" {
			return name, true
		}
		return definedName, true
	}
	return name, false
}

// transformName handles identifier name conversion following these rules:
// 1. First checks if the name exists in predefined mapping (in typeMap of llcppg.cfg)
// 2. If not in predefined mapping, applies the transform function
// 3. Before applying the transform function, removes specified prefixes (obtained via trimPrefixes)
//
// Parameters:
//   - name: Original C/C++ identifier name
//   - transform: Name transformation function (like names.PubName or names.ExportName)
//
// Returns:
//   - Transformed identifier name
func (p *NodeConverter) transformName(cname string, transform NameMethod) string {
	if definedName, ok := p.definedName(cname); ok {
		return definedName
	}
	return transform(name.RemovePrefixedName(cname, p.conf.TrimPrefixes))
}

func (p *NodeConverter) declName(cname string) string {
	return p.transformName(cname, name.PubName)
}

func (p *NodeConverter) constName(cname string) string {
	return p.transformName(cname, name.ExportName)
}

type nodeKind int

const (
	FuncDecl nodeKind = iota + 1
	TypeDecl
	TypedefDecl
	EnumTypeDecl
	EnumItem
	Macro
)

type Node struct {
	name string
	kind nodeKind
}

type ProcessSymbol struct {
	// not same node can have same name,so use the Node as key
	info  map[Node]string
	count map[string]int
}

func NewProcessSymbol() *ProcessSymbol {
	return &ProcessSymbol{
		info:  make(map[Node]string),
		count: make(map[string]int),
	}
}

func (p *ProcessSymbol) Lookup(node Node) (string, bool) {
	pubName, ok := p.info[node]
	return pubName, ok
}

func (p *ProcessSymbol) Register(node Node, pubName string) string {
	p.count[pubName]++
	count := p.count[pubName]
	pubName = name.SuffixCount(pubName, count)
	p.info[node] = pubName
	return pubName
}

// Write all files in the package to the output directory
func writePkg(pkg *gogen.Package, outDir string) error {
	var errs errors.List
	pkg.ForEachFile(func(fname string, _ *gogen.File) {
		if fname != "" { // gogen default fname
			outFile := filepath.Join(outDir, fname)
			e := pkg.WriteFile(outFile, fname)
			if e != nil {
				errs.Add(e)
			}
		}
	})
	return errs.ToError()
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func prepareEnv(outputDir string, deps []string, modulePath string) error {
	err := os.MkdirAll(outputDir, 0744)
	if err != nil {
		return err
	}

	err = os.Chdir(outputDir)
	if err != nil {
		return err
	}

	return cl.ModInit(deps, outputDir, modulePath)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: gogensig [-v|-cfg|-mod] [sigfetch-file]")
}
