package convert

import (
	"errors"
	"log"
	"strings"

	"github.com/goplus/llcppg/ast"
	cfg "github.com/goplus/llcppg/cmd/gogensig/config"
	"github.com/goplus/llcppg/cmd/gogensig/dbg"
	"github.com/goplus/llcppg/cmd/gogensig/visitor"
	cppgtypes "github.com/goplus/llcppg/types"
)

type AstConvert struct {
	*visitor.BaseDocVisitor
	Pkg       *Package
	visitDone func(pkg *Package, incPath string)
}

type AstConvertConfig struct {
	PkgName   string
	SymbFile  string // llcppg.symb.json
	CfgFile   string // llcppg.cfg
	PubFile   string // llcppg.pub
	OutputDir string
}

type ConverterConfig = AstConvertConfig

func NewAstConvert(config *AstConvertConfig) (*AstConvert, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}
	p := new(AstConvert)
	p.BaseDocVisitor = visitor.NewBaseDocVisitor(p)
	symbTable, err := cfg.NewSymbolTable(config.SymbFile)
	if err != nil {
		if dbg.GetDebugError() {
			log.Printf("Can't get llcppg.symb.json from %s Use empty table\n", config.SymbFile)
		}
		symbTable = cfg.CreateSymbolTable([]cfg.SymbolEntry{})
	}

	conf, err := cfg.GetCppgCfgFromPath(config.CfgFile)
	if err != nil {
		if dbg.GetDebugError() {
			log.Printf("Cant get llcppg.cfg from %s Use empty config\n", config.CfgFile)
		}
		conf = &cppgtypes.Config{}
	}

	pubs, err := cfg.GetPubFromPath(config.PubFile)
	if err != nil {
		return nil, err
	}

	pkg := NewPackage(&PackageConfig{
		PkgBase: PkgBase{
			PkgPath:  ".",
			CppgConf: conf,
			Pubs:     pubs,
		},
		Name:        config.PkgName,
		OutputDir:   config.OutputDir,
		SymbolTable: symbTable,
	})
	p.Pkg = pkg
	return p, nil
}

func (p *AstConvert) SetVisitDone(fn func(pkg *Package, incPath string)) {
	p.visitDone = fn
}

func (p *AstConvert) WriteLinkFile() {
	p.Pkg.WriteLinkFile()
}

func (p *AstConvert) WritePubFile() {
	p.Pkg.WritePubFile()
}

func (p *AstConvert) VisitFuncDecl(funcDecl *ast.FuncDecl) {
	err := p.Pkg.NewFuncDecl(funcDecl)
	if err != nil {
		if dbg.GetDebugError() {
			log.Printf("NewFuncDecl %s Fail: %s\n", funcDecl.Name.Name, err.Error())
		}
	}
}

func (p *AstConvert) VisitMacro(macro *ast.Macro) {
	err := p.Pkg.NewMacro(macro)
	if err != nil {
		log.Printf("NewMacro %s Fail: %s\n", macro.Name, err.Error())
	}
}

/*
//TODO
func (p *AstConvert) VisitClass(className *ast.Ident, fields *ast.FieldList, typeDecl *ast.TypeDecl) {
	fmt.Printf("visit class %s\n", className.Name)
	p.pkg.NewTypeDecl(typeDecl)
}

func (p *AstConvert) VisitMethod(className *ast.Ident, method *ast.FuncDecl, typeDecl *ast.TypeDecl) {
	fmt.Printf("visit method %s of %s\n", method.Name.Name, className.Name)
}*/

func (p *AstConvert) VisitStruct(structName *ast.Ident, fields *ast.FieldList, typeDecl *ast.TypeDecl) {
	// https://github.com/goplus/llcppg/issues/66 ignore unexpected struct name
	// Union (unnamed at /usr/local/Cellar/msgpack/6.0.2/include/msgpack/object.h:75:9)
	if strings.ContainsAny(structName.Name, ":\\/") {
		if dbg.GetDebugLog() {
			log.Println("structName", structName.Name, "ignored to convert")
		}
		return
	}
	err := p.Pkg.NewTypeDecl(typeDecl)
	if typeDecl.Name == nil {
		log.Printf("NewTypeDecl anonymous struct skipped")
	}
	if err != nil {
		if name := typeDecl.Name; name != nil {
			log.Printf("NewTypeDecl %s Fail: %s\n", name.Name, err.Error())
		}
	}
}

func (p *AstConvert) VisitUnion(unionName *ast.Ident, fields *ast.FieldList, typeDecl *ast.TypeDecl) {
	p.VisitStruct(unionName, fields, typeDecl)
}

func (p *AstConvert) VisitEnumTypeDecl(enumTypeDecl *ast.EnumTypeDecl) {
	err := p.Pkg.NewEnumTypeDecl(enumTypeDecl)
	if err != nil {
		if name := enumTypeDecl.Name; name != nil {
			log.Printf("NewEnumTypeDecl %s Fail: %s\n", name.Name, err.Error())
		} else {
			log.Printf("NewEnumTypeDecl anonymous Fail: %s\n", err.Error())
		}
	}
}

func (p *AstConvert) VisitTypedefDecl(typedefDecl *ast.TypedefDecl) {
	err := p.Pkg.NewTypedefDecl(typedefDecl)
	if err != nil {
		log.Printf("NewTypedefDecl %s Fail: %s\n", typedefDecl.Name.Name, err.Error())
	}
}

func (p *AstConvert) VisitStart(path string, incPath string, isSys bool) {
	inPkgIncPath := false
	incPaths, notFounds, err := p.Pkg.GetIncPaths()
	if len(notFounds) > 0 {
		log.Println("failed to find some include paths: \n", notFounds)
		if err != nil {
			log.Println("failed to get any include paths: \n", err.Error())
		}
	}
	for _, includePath := range incPaths {
		if includePath == path {
			inPkgIncPath = true
			break
		}
	}
	p.Pkg.SetCurFile(&HeaderFile{
		File:         path,
		IncPath:      incPath,
		IsHeaderFile: true,
		InCurPkg:     inPkgIncPath,
		IsSys:        isSys,
	})
}

func (p *AstConvert) VisitDone(incPath string) {
	if p.visitDone != nil {
		p.visitDone(p.Pkg, incPath)
	}
}

func (p *AstConvert) WritePkgFiles() {
	err := p.Pkg.WritePkgFiles()
	if err != nil {
		log.Panicf("WritePkgFiles: %v", err)
	}
}

type Converter struct {
	Types   *Types
	FileSet []*ast.FileEntry
	Pkg     *Package

	files map[string]*ast.FileEntry
}

func NewConverter(config *ConverterConfig) (*Converter, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}
	symbTable, err := cfg.NewSymbolTable(config.SymbFile)
	if err != nil {
		if dbg.GetDebugError() {
			log.Printf("Can't get llcppg.symb.json from %s Use empty table\n", config.SymbFile)
		}
		symbTable = cfg.CreateSymbolTable([]cfg.SymbolEntry{})
	}

	conf, err := cfg.GetCppgCfgFromPath(config.CfgFile)
	if err != nil {
		if dbg.GetDebugError() {
			log.Printf("Cant get llcppg.cfg from %s Use empty config\n", config.CfgFile)
		}
		conf = &cppgtypes.Config{}
	}

	pubs, err := cfg.GetPubFromPath(config.PubFile)
	if err != nil {
		return nil, err
	}

	pkg := NewPackage(&PackageConfig{
		PkgBase: PkgBase{
			PkgPath:  ".",
			CppgConf: conf,
			Pubs:     pubs,
		},
		Name:        config.PkgName,
		OutputDir:   config.OutputDir,
		SymbolTable: symbTable,
	})
	return &Converter{
		Types: NewTypes(),
		files: make(map[string]*ast.FileEntry),
		Pkg:   pkg,
	}, nil
}

func (p *Converter) Start() error {
	order, err := p.BuildOrder()
	if err != nil {
		return err
	}
	p.Process(order)
	p.Pkg.WritePkgFiles()
	p.Pkg.WriteLinkFile()
	p.Pkg.WritePubFile()
	return nil
}

func (p *Converter) Collect(files []*ast.FileEntry) {
	p.FileSet = files
	for _, file := range files {
		// for lookup file by path
		p.files[file.Path] = file
		for _, decl := range file.Doc.Decls {
			switch d := decl.(type) {
			case *ast.TypeDecl:
				p.Types.Register(d.Name.USR, d)
			case *ast.TypedefDecl:
				p.Types.Register(d.Name.USR, d)
			case *ast.EnumTypeDecl:
				p.Types.Register(d.Name.USR, d)
			case *ast.FuncDecl:
				p.Types.Register(d.Name.USR, d)
			}
		}
	}
	p.CollectDeps(files)
}

func (p *Converter) Process(orderedUSR []string) error {
	// Register Decl without type to keep type order
	for _, file := range p.FileSet {
		p.Pkg.SetCurFile(Hfile(p.Pkg, file))
		for _, decl := range file.Doc.Decls {
			switch decl := decl.(type) {
			case *ast.TypedefDecl:
				if dbg.GetDebugLog() {
					log.Printf("Registering typedef decl: %s", decl.Name.USR)
				}
				p.Pkg.RegisterDecl(decl.Name.USR)
			case *ast.EnumTypeDecl:
				if dbg.GetDebugLog() {
					log.Printf("Registering enum decl: %s", decl.Name.USR)
				}
				p.Pkg.RegisterEnumDecl(decl.Name.USR)
			case *ast.TypeDecl:
				if dbg.GetDebugLog() {
					log.Printf("Registering type decl: %s", decl.Name.USR)
				}
				p.Pkg.RegisterDecl(decl.Name.USR)
				// TODO: register func decl
				// But gogen now could not only register func decl node in ast.
			}
		}
	}

	if dbg.GetDebugLog() {
		log.Printf("Processing decls: %v", orderedUSR)
	}
	for i, usr := range orderedUSR {
		if dbg.GetDebugLog() {
			log.Printf("Processing USR[%d/%d]: %s", i+1, len(orderedUSR), usr)
		}
		typ, ok := p.Types.Lookup(usr)
		if !ok {
			if dbg.GetDebugLog() {
				log.Printf("Type not found for USR: %s in Types.Defs", usr)
			}
			continue
		}

		switch decl := typ.(type) {
		case *ast.TypeDecl:
			p.Pkg.SetCurFile(Hfile(p.Pkg, p.files[decl.DeclBase.Loc.File]))
			if err := p.Pkg.ConvertTypeDecl(decl); err != nil {
				log.Printf("ConvertTypeDecl %s Fail: %s", decl.Name.Name, err.Error())
			}
		case *ast.EnumTypeDecl:
			p.Pkg.SetCurFile(Hfile(p.Pkg, p.files[decl.DeclBase.Loc.File]))
			if err := p.Pkg.ConvertEnumTypeDecl(decl); err != nil {
				log.Printf("ConvertEnumTyleDecl %s Fail: %s", decl.Name.Name, err.Error())
			}
		case *ast.TypedefDecl:
			p.Pkg.SetCurFile(Hfile(p.Pkg, p.files[decl.DeclBase.Loc.File]))
			if err := p.Pkg.ConvertTypedefDecl(decl); err != nil {
				log.Printf("ConvertTypedefDecl %s Fail: %s", decl.Name.Name, err.Error())
			}
		case *ast.FuncDecl:
			p.Pkg.SetCurFile(Hfile(p.Pkg, p.files[decl.DeclBase.Loc.File]))
			if err := p.Pkg.NewFuncDecl(decl); err != nil {
				log.Printf("ConvertFuncDecl %s Fail: %s", decl.Name.Name, err.Error())
			}
		}
	}
	return nil
}

func (p *Converter) CollectDeps(files []*ast.FileEntry) {
	for _, file := range files {
		for _, decl := range file.Doc.Decls {
			switch d := decl.(type) {
			case *ast.TypeDecl:
				p.depType(d.Name.USR, d.Type)
			case *ast.TypedefDecl:
				p.depType(d.Name.USR, d.Type)
			case *ast.EnumTypeDecl:
				p.depType(d.Name.USR, d.Type)
			case *ast.FuncDecl:
				p.depType(d.Name.USR, d.Type)
			}
		}
	}
}

func (p *Converter) depType(usr string, expr ast.Expr) {
	switch t := expr.(type) {
	case *ast.BuiltinType:
	case *ast.Variadic:
	case *ast.PointerType:
		p.depType(usr, t.X)
	case *ast.Ident, *ast.ScopingExpr, *ast.TagExpr:
		p.depIdentRefer(usr, t)
	case *ast.Field:
		p.depType(usr, t.Type)
	case *ast.FieldList:
		for _, field := range t.List {
			p.depType(usr, field.Type)
		}
	case *ast.FuncType:
		p.depType(usr, t.Params)
		p.depType(usr, t.Ret)
	case *ast.ArrayType:
		p.depType(usr, t.Elt)
	case *ast.RecordType:
		for _, field := range t.Fields.List {
			p.depType(usr, field.Type)
		}
	}
}

func (p *Converter) depIdentRefer(usr string, expr ast.Expr) {
	switch t := expr.(type) {
	case *ast.Ident:
		p.Types.RecordDep(usr, t.USR)
	case *ast.ScopingExpr:
		p.depIdentRefer(usr, t.X)
	case *ast.TagExpr:
		p.depIdentRefer(usr, t.Name)
	}
}

func (p *Converter) BuildOrder() ([]string, error) {
	visited := make(map[string]bool)
	temp := make(map[string]bool)
	var order []string

	var visit func(string) error
	visit = func(usr string) error {
		// nested dependency avoid recursive
		if temp[usr] {
			return nil
		}
		if visited[usr] {
			return nil
		}

		temp[usr] = true
		for _, depUSR := range p.Types.Deps[usr] {
			if err := visit(depUSR); err != nil {
				return err
			}
		}

		temp[usr] = false
		visited[usr] = true
		order = append(order, usr)
		return nil
	}

	for usr := range p.Types.Defs {
		if !visited[usr] {
			if err := visit(usr); err != nil {
				return nil, err
			}
		}
	}

	return order, nil
}

func Hfile(pkg *Package, file *ast.FileEntry) *HeaderFile {
	inPkgIncPath := false
	incPaths, notFounds, err := pkg.GetIncPaths()
	if len(notFounds) > 0 {
		log.Println("failed to find some include paths: \n", notFounds)
		if err != nil {
			log.Println("failed to get any include paths: \n", err.Error())
		}
	}
	for _, includePath := range incPaths {
		if includePath == file.Path {
			inPkgIncPath = true
			break
		}
	}
	return &HeaderFile{
		File:         file.Path,
		IncPath:      file.IncPath,
		IsHeaderFile: true,
		InCurPkg:     inPkgIncPath,
		IsSys:        file.IsSys,
	}
}
