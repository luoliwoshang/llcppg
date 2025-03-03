/*
This file is used to convert type from ast type to types.Type
*/
package convert

import (
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"log"
	"unsafe"

	"github.com/goplus/gogen"
	"github.com/goplus/llcppg/_xtool/llcppsymg/names"
	"github.com/goplus/llcppg/ast"
	"github.com/goplus/llcppg/cmd/gogensig/config"
	"github.com/goplus/llcppg/cmd/gogensig/convert/sizes"
	"github.com/goplus/llcppg/cmd/gogensig/errs"
)

type TypeContext int

var (
	ErrTypeConv = errors.New("error convert type")
)

const (
	Normal TypeContext = iota
	Param              // In function parameter context
	Record             // In record field context
)

type TypeConv struct {
	pkg         *Package
	types       *types.Package
	symbolTable *config.SymbolTable // llcppg.symb.json
	typeMap     *BuiltinTypeMap
	ctx         TypeContext
}

type TypeConfig struct {
	Package     *Package
	TypeMap     *BuiltinTypeMap
	SymbolTable *config.SymbolTable
}

func NewConv(conf *TypeConfig) *TypeConv {
	typeConv := &TypeConv{
		symbolTable: conf.SymbolTable,
		typeMap:     conf.TypeMap,
		pkg:         conf.Package,
		types:       conf.Package.p.Types,
	}
	return typeConv
}

// Convert ast.Expr to types.Type
func (p *TypeConv) ToType(expr ast.Expr) (types.Type, error) {
	switch t := expr.(type) {
	case *ast.BuiltinType:
		typ, err := p.typeMap.FindBuiltinType(*t)
		return typ, err
	case *ast.PointerType:
		return p.handlePointerType(t)
	case *ast.ArrayType:
		return p.handleArrayType(t)
	case *ast.FuncType:
		return p.ToSignature(t, nil)
	case *ast.Ident, *ast.ScopingExpr, *ast.TagExpr:
		return p.handleIdentRefer(expr)
	case *ast.Variadic:
		return types.NewSlice(gogen.TyEmptyInterface), nil
	case *ast.RecordType:
		return p.RecordTypeToStruct(t)
	default:
		return nil, fmt.Errorf("%w: unsupported type %T", ErrTypeConv, expr)
	}
}

func (p *TypeConv) handleArrayType(t *ast.ArrayType) (types.Type, error) {
	elemType, err := p.ToType(t.Elt)
	if err != nil {
		return nil, fmt.Errorf("error convert elem type: %w", err)
	}
	if p.ctx == Param {
		// array in the parameter,ignore the len,convert as pointer
		return types.NewPointer(elemType), nil
	}

	if t.Len == nil {
		return nil, fmt.Errorf("%s", "unsupport field with array without length")
	}

	len, err := Expr(t.Len).ToInt()
	if err != nil {
		return nil, fmt.Errorf("%s", "can't determine the array length")
	}

	return types.NewArray(elemType, int64(len)), nil
}

// - void* -> c.Pointer
// - Function pointers -> Function types (pointer removed)
// - Other cases -> Pointer to the base type
func (p *TypeConv) handlePointerType(t *ast.PointerType) (types.Type, error) {
	baseType, err := p.ToType(t.X)
	if err != nil {
		return nil, fmt.Errorf("error convert baseType: %w", err)
	}
	// void * -> c.Pointer
	// todo(zzy):alias visit the origin type unsafe.Pointer,c.Pointer is better
	if p.typeMap.IsVoidType(baseType) {
		return p.typeMap.CType("Pointer"), nil
	}

	if p.ctx == Param {
		if named, ok := baseType.(*types.Named); ok {
			if _, ok := named.Underlying().(*types.Signature); ok {
				return baseType, nil
			}
		}
	}

	if baseFuncType, ok := baseType.(*types.Signature); ok {
		if p.ctx == Record {
			return p.typeMap.CType("Pointer"), nil
		}
		return baseFuncType, nil
	}

	return types.NewPointer(baseType), nil
}

func (p *TypeConv) handleIdentRefer(t ast.Expr) (types.Type, error) {
	lookup := func(name string) types.Type {
		// For types defined in other packages, they should already be in current scope
		// We don't check for types.Named here because the type returned from ConvertType
		// for aliases like int8_t might be a built-in type (e.g., int8),

		var typ types.Type
		obj := gogen.Lookup(p.types.Scope(), name)
		if obj == nil {
			// in third hfile but not have converted go type
			if path, ok := p.pkg.locMap.Lookup(name); ok {
				log.Panicf("convert %s first, declare its converted package in llcppg.cfg deps for load [%s]. See: https://github.com/goplus/llcppg?tab=readme-ov-file#dependency", path, name)
			} else {
				// implicit forward decl
				decl := p.pkg.handleImplicitForwardDecl(name)
				typ = decl.Type()
			}
		} else {
			typ = obj.Type()
		}

		if p.ctx == Record {
			if named, ok := typ.(*types.Named); ok {
				if _, ok := named.Underlying().(*types.Signature); ok {
					return p.typeMap.CType("Pointer")
				}
			}
		}
		return typ
	}
	switch t := t.(type) {
	case *ast.Ident:
		typ := lookup(t.Name)
		return typ, nil
	case *ast.ScopingExpr:
		// todo(zzy)
	case *ast.TagExpr:
		// todo(zzy):scoping
		if ident, ok := t.Name.(*ast.Ident); ok {
			typ := lookup(ident.Name)
			return typ, nil
		}
		// todo(zzy):scoping expr
	}
	return nil, errs.NewUnsupportedReferError(t)
}

func (p *TypeConv) ToSignature(funcType *ast.FuncType, recv *types.Var) (*types.Signature, error) {
	ctx := p.ctx
	p.ctx = Param
	defer func() { p.ctx = ctx }()
	var params *types.Tuple
	var variadic bool
	var err error
	if recv != nil {
		params, variadic, err = p.fieldListToParams(&ast.FieldList{List: funcType.Params.List[1:]})
	} else {
		params, variadic, err = p.fieldListToParams(funcType.Params)
	}
	if err != nil {
		return nil, err
	}
	results, err := p.retToResult(funcType.Ret)
	if err != nil {
		return nil, err
	}
	return types.NewSignatureType(recv, nil, nil, params, results, variadic), nil
}

// Convert ast.FieldList to types.Tuple (Function Param)
func (p *TypeConv) fieldListToParams(params *ast.FieldList) (*types.Tuple, bool, error) {
	if params == nil {
		return types.NewTuple(), false, nil
	}

	hasNamedParam := false
	for _, field := range params.List {
		if field == nil {
			continue
		}
		if len(field.Names) > 0 {
			hasNamedParam = true
			break
		}
		if _, ok := field.Type.(*ast.Variadic); ok {
			hasNamedParam = true
			break
		}
	}

	vars, err := p.fieldListToVars(params, hasNamedParam)
	if err != nil {
		return nil, false, err
	}
	variadic := false
	if len(params.List) > 0 {
		lastField := params.List[len(params.List)-1]
		if _, ok := lastField.Type.(*ast.Variadic); ok {
			variadic = true
		}
	}
	return types.NewTuple(vars...), variadic, nil
}

// Execute the ret in FuncType
func (p *TypeConv) retToResult(ret ast.Expr) (*types.Tuple, error) {
	typ, err := p.ToType(ret)
	if err != nil {
		return nil, fmt.Errorf("error convert return type: %w", err)
	}
	if typ != nil && !p.typeMap.IsVoidType(typ) {
		// in c havent multiple return
		return types.NewTuple(types.NewVar(token.NoPos, p.types, "", typ)), nil
	}
	return types.NewTuple(), nil
}

// Convert ast.FieldList to []types.Var
func (p *TypeConv) fieldListToVars(params *ast.FieldList, hasNamedParam bool) ([]*types.Var, error) {
	var vars []*types.Var
	if params == nil || params.List == nil {
		return vars, nil
	}

	for index, field := range params.List {
		fieldVar, err := p.fieldToVar(field, hasNamedParam, index)
		if err != nil {
			return nil, err
		}
		if fieldVar != nil {
			vars = append(vars, fieldVar)
		}
	}
	return vars, nil
}

// todo(zzy): use  Unused [unsafe.Sizeof(0)]byte in the source code
func (p *TypeConv) defaultRecordField() []*types.Var {
	return []*types.Var{
		types.NewVar(token.NoPos, p.types, "Unused", types.NewArray(types.Typ[types.Byte], int64(unsafe.Sizeof(0)))),
	}
}

func (p *TypeConv) fieldToVar(field *ast.Field, hasNamedParam bool, argIndex int) (*types.Var, error) {
	if field == nil {
		return nil, fmt.Errorf("%w: unexpected nil field", ErrTypeConv)
	}

	//field without name
	var name string
	if len(field.Names) > 0 {
		name = field.Names[0].Name
	} else if hasNamedParam {
		name = fmt.Sprintf("__llgo_arg_%d", argIndex)
	}

	typ, err := p.ToType(field.Type)
	if err != nil {
		return nil, err
	}

	if p.ctx == Record {
		name = getFieldName(name)
	} else {
		_, isVariadic := field.Type.(*ast.Variadic)
		if isVariadic && hasNamedParam {
			name = "__llgo_va_list"
		} else {
			name = avoidKeyword(name)
		}
	}
	return types.NewVar(token.NoPos, p.types, name, typ), nil
}

func (p *TypeConv) RecordTypeToStruct(recordType *ast.RecordType) (types.Type, error) {
	ctx := p.ctx
	p.ctx = Record
	defer func() { p.ctx = ctx }()
	var fields []*types.Var
	flds, err := p.fieldListToVars(recordType.Fields, false)
	if err != nil {
		return nil, err
	}
	if recordType.Tag != ast.Union {
		fields = flds
	} else {
		var maxFld *types.Var
		maxSize := int64(0)
		for i := len(flds) - 1; i >= 0; i-- {
			fld := flds[i]
			t := fld.Type()
			size := sizes.Sizeof(t)
			if size >= maxSize {
				maxSize = size
				maxFld = fld
			}
		}
		if maxFld != nil {
			fields = []*types.Var{maxFld}
		}
	}
	return types.NewStruct(fields, nil), nil
}

func (p *TypeConv) ToDefaultEnumType() types.Type {
	return p.typeMap.CType("Int")
}

// todo(zzy): Current forward declaration detection is imprecise
// It incorrectly treats both empty struct `struct a {}` and forward declaration `struct a` as the same
// by only checking if Fields.List is empty
// Should use recordType == nil to identify forward declarations, which requires llcppsigfetch support
func (p *TypeConv) inComplete(recordType *ast.RecordType) bool {
	return recordType.Fields != nil && len(recordType.Fields.List) == 0
}

func (p *TypeConv) LookupSymbol(mangleName config.MangleNameType) (*GoFuncSpec, error) {
	if p.symbolTable == nil {
		return nil, fmt.Errorf("symbol table not initialized")
	}
	e, err := p.symbolTable.LookupSymbol(mangleName)
	if err != nil {
		return nil, err
	}
	return NewGoFuncSpec(e.GoName), nil
}

// The field name should be public if it's a record field
func getFieldName(name string) string {
	return names.PubName(name)
}

func avoidKeyword(name string) string {
	if token.IsKeyword(name) {
		return name + "_"
	}
	return name
}

func substObj(pkg *types.Package, scope *types.Scope, origName string, real types.Object) {
	old := scope.Insert(gogen.NewSubst(token.NoPos, pkg, origName, real))
	if old != nil {
		if t, ok := old.Type().(*gogen.TySubst); ok {
			t.Real = real
		} else {
			log.Panicln(origName, "redefined")
		}
	}
}
