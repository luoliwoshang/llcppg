package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type MangleNameType = string

type CppNameType = string

type GoNameType = string

type SymbolEntry struct {
	MangleName MangleNameType `json:"mangle"`
	CppName    CppNameType    `json:"c++"`
	GoName     GoNameType     `json:"go"`
}

type SymbolTable struct {
	t map[MangleNameType]SymbolEntry
}

// llcppg.symb.json
func NewSymbolTable(filePath string) (*SymbolTable, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var symbs []SymbolEntry
	err = json.Unmarshal(bytes, &symbs)
	if err != nil {
		return nil, err
	}
	return CreateSymbolTable(symbs), nil
}
func CreateSymbolTable(symbs []SymbolEntry) *SymbolTable {
	symbolTable := &SymbolTable{
		t: make(map[MangleNameType]SymbolEntry),
	}
	for _, symb := range symbs {
		symbolTable.t[symb.MangleName] = symb
	}
	return symbolTable
}

func (t *SymbolTable) LookupSymbol(name MangleNameType) (*SymbolEntry, error) {
	symbol, ok := t.t[name]
	if ok {
		return &symbol, nil
	}
	return nil, fmt.Errorf("symbol %s not found", name)
}
