package config_test

import (
	"path"
	"testing"

	"github.com/goplus/llcppg/cmd/gogensig/config"
	llcppg "github.com/goplus/llcppg/config"
)

func TestLookupSymbolOK(t *testing.T) {
	table, err := config.NewSymbolTable(path.Join("./_testinput", llcppg.LLCPPG_SYMB))
	if err != nil {
		t.Fatal(err)
	}
	entry, err := table.LookupSymbol("_ZNK9INIReader10GetBooleanERKNSt3__112basic_stringIcNS0_11char_traitsIcEENS0_9allocatorIcEEEES8_b")
	if err != nil {
		t.Fatal(err)
	}
	const expectCppName = "INIReader::GetBoolean(std::__1::basic_string<char, std::__1::char_traits<char>, std::__1::allocator<char>> const&, std::__1::basic_string<char, std::__1::char_traits<char>, std::__1::allocator<char>> const&, bool) const"
	const expectGoName = "(*Reader).GetBoolean"

	if entry.CppName != expectCppName {
		t.Fatalf("expect %s, got %s", expectCppName, entry.CppName)
	}
	if entry.GoName != expectGoName {
		t.Fatalf("expect %s, got %s", expectGoName, entry.GoName)
	}
}

func TestLookupSymbolError(t *testing.T) {
	_, err := config.NewSymbolTable("./_testinput/llcppg.symb.txt")
	if err == nil {
		t.Error("expect error")
	}
	table, err := config.NewSymbolTable(path.Join("./_testinput", llcppg.LLCPPG_SYMB))
	if err != nil {
		t.Fatal(err)
	}
	lookupSymbs := []string{
		"_ZNK9INIReader10GetBooleanERKNSt3__112basic_stringIcNS0_11char_traitsIcEENS0_9allocatorIcEEEES8_bXXX",
		"",
	}
	for _, lookupSymbol := range lookupSymbs {
		_, err := table.LookupSymbol(lookupSymbol)
		if err == nil {
			t.Error("expect error")
		}
	}
}
