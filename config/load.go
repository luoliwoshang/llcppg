package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func GetConfFromStdin() (conf Config, err error) {
	return ConfigFromReader(os.Stdin)
}

func GetConfFromFile(cfgFile string) (conf Config, err error) {
	fileReader, err := os.Open(cfgFile)
	if err != nil {
		return
	}
	defer fileReader.Close()

	return ConfigFromReader(fileReader)
}

func ConfigFromReader(reader io.Reader) (Config, error) {
	var config Config

	if err := json.NewDecoder(reader).Decode(&config); err != nil {
		return Config{}, err
	}

	return config, nil
}

func GetSymTableFromFile(symFile string) (symTable *SymTable, err error) {
	bytes, err := os.ReadFile(symFile)
	if err != nil {
		return nil, err
	}
	var syms []SymbolInfo
	err = json.Unmarshal(bytes, &syms)
	if err != nil {
		return nil, err
	}
	return NewSymTable(syms), nil
}

type SymTable struct {
	t map[string]SymbolInfo // mangle -> SymbolInfo
}

func NewSymTable(syms []SymbolInfo) *SymTable {
	symTable := &SymTable{
		t: make(map[string]SymbolInfo),
	}
	for _, sym := range syms {
		symTable.t[sym.Mangle] = sym
	}
	return symTable
}

func (t *SymTable) LookupSymbol(mangle string) (*SymbolInfo, error) {
	symbol, ok := t.t[mangle]
	if ok {
		return &symbol, nil
	}
	return nil, fmt.Errorf("symbol %s not found", mangle)
}
