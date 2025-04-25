package names

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// NameMapper handles name mapping and uniqueness for Go symbols
type NameMapper struct {
	count   map[string]int    // tracks count of each public name for uniqueness
	mapping map[string]string // maps original c names to Go names,like: foo(in c) -> Foo(in go)
}

func NewNameMapper() *NameMapper {
	return &NameMapper{
		count:   make(map[string]int),
		mapping: make(map[string]string),
	}
}

type ConvertKind uint

const (
	None ConvertKind = iota
	ToCamel
	ToExport
)

// returns a unique Go name for an original name
// For every go name, it will be unique.
func (m *NameMapper) GetUniqueGoName(name string, trimPrefixes []string, convertKind ConvertKind) (string, bool) {
	pubName, exist := m.genGoName(name, trimPrefixes, convertKind)
	if exist {
		return pubName, pubName != name
	}

	count := m.count[pubName]
	m.count[pubName]++
	if count > 0 {
		pubName = fmt.Sprintf("%s__%d", pubName, count)
	}

	return pubName, pubName != name
}

func (m *NameMapper) GetUniqueFuncName(name string, goFuncName string) (string, bool) {
	var funcName string
	var exist bool
	goName, exists := m.mapping[name]
	if exists {
		exist = true
		if goName == "" {
			funcName = name
		}
		funcName = goName
	}

	if exist {
		return funcName, funcName != name
	}

	var pubName string
	count := m.count[goFuncName]
	m.count[goFuncName]++
	if count > 0 {
		pubName = fmt.Sprintf("%s__%d", goFuncName, count)
	} else {
		pubName = goFuncName
	}

	return pubName, pubName != name
}

// returns the Go name for an original name,if the name is already mapped,return the mapped name
func (m *NameMapper) genGoName(name string, trimPrefixes []string, convertKind ConvertKind) (string, bool) {
	if goName, exists := m.mapping[name]; exists {
		if goName == "" {
			return name, true
		}
		return goName, true
	}
	name = removePrefixedName(name, trimPrefixes)
	switch convertKind {
	case ToCamel:
		return PubName(name), false
	case ToExport:
		return ExportName(name), false
	case None:
		return name, false
	default:
		panic("unexpect convert kind")
	}
}

func (m *NameMapper) SetMapping(originName, newName string) {
	value := ""
	if originName != newName {
		value = newName
	}
	m.mapping[originName] = value
}

func (m *NameMapper) Lookup(name string) int {
	return m.count[name]
}

func GoName(name string, trimPrefixes []string, inCurPkg bool) string {
	if inCurPkg {
		name = removePrefixedName(name, trimPrefixes)
	}
	return PubName(name)
}

func removePrefixedName(name string, trimPrefixes []string) string {
	if len(trimPrefixes) == 0 {
		return name
	}
	for _, prefix := range trimPrefixes {
		if strings.HasPrefix(name, prefix) {
			return strings.TrimPrefix(name, prefix)
		}
	}
	return name
}

func PubName(name string) string {
	if len(name) == 0 {
		return name
	}
	baseName := strings.Trim(name, "_")
	if len(baseName) == 0 {
		return "X" + name
	}
	prefix := preUScore(name)
	suffix := sufUScore(name)

	if len(prefix) != 0 || unicode.IsDigit(rune(baseName[0])) {
		return "X" + prefix + ToCamelCase(baseName, false) + suffix
	}
	return ToCamelCase(baseName, true) + suffix
}

func sufUScore(name string) string {
	return strings.Repeat("_", len(name)-len(strings.TrimRight(name, "_")))
}

func preUScore(name string) string {
	return strings.Repeat("_", len(name)-len(strings.TrimLeft(name, "_")))
}

func ToCamelCase(s string, firstPartUpper bool) string {
	parts := strings.Split(s, "_")
	result := []string{}
	for i, part := range parts {
		if i == 0 && !firstPartUpper {
			result = append(result, part)
			continue
		}
		if len(part) > 0 {
			result = append(result, strings.ToUpper(part[:1])+part[1:])
		}
	}
	return strings.Join(result, "")
}

// Only Make it Public,no turn to other camel method
func ExportName(name string) string {
	fChar := name[0]
	if fChar == '_' || unicode.IsDigit(rune(fChar)) {
		return "X" + name
	}
	return UpperFirst(name)
}

func UpperFirst(name string) string {
	return strings.ToUpper(name[:1]) + name[1:]
}

// /path/to/foo.h -> foo.go
// /path/to/_intptr.h -> X_intptr.go
func HeaderFileToGo(incPath string) string {
	_, fileName := filepath.Split(incPath)
	ext := filepath.Ext(fileName)
	if len(ext) > 0 {
		fileName = strings.TrimSuffix(fileName, ext)
	}
	if strings.HasPrefix(fileName, "_") {
		fileName = "X" + fileName
	}
	return fileName + ".go"
}
