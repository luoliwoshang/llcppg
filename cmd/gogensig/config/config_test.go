package config_test

import (
	"os"
	"path"
	"path/filepath"
	"reflect"
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

func TestGetCppgCfgFromPath(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Prepare valid config file
	validConfigPath := filepath.Join(tempDir, "valid_config.cfg")
	validConfigContent := `{
		"name": "lua",
		"cflags": "$(pkg-config --cflags lua5.4)",
		"include": ["litelua.h"],
		"libs": "$(pkg-config --libs lua5.4)",
		"trimPrefixes": ["lua_"],
		"cplusplus": false
	}`
	err = os.WriteFile(validConfigPath, []byte(validConfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create valid config file: %v", err)
	}

	t.Run("Successfully parse config file", func(t *testing.T) {
		cfg, err := config.GetCppgCfgFromPath(validConfigPath)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if cfg == nil {
			t.Fatal("Expected non-nil config")
		}

		expectedConfig := llcppg.NewDefault()
		expectedConfig.Name = "lua"
		expectedConfig.CFlags = "$(pkg-config --cflags lua5.4)"
		expectedConfig.Include = []string{"litelua.h"}
		expectedConfig.Libs = "$(pkg-config --libs lua5.4)"
		expectedConfig.TrimPrefixes = []string{"lua_"}

		if !reflect.DeepEqual(cfg, expectedConfig) {
			t.Errorf("Parsed config does not match expected config.\nGot: %+v\nWant: %+v", cfg, expectedConfig)
		}
	})

	t.Run("File not found", func(t *testing.T) {
		_, err := config.GetCppgCfgFromPath(filepath.Join(tempDir, "nonexistent_file.cfg"))
		if err == nil {
			t.Error("Expected error for non-existent file, got nil")
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		invalidJSONPath := filepath.Join(tempDir, "invalid_config.cfg")
		err := os.WriteFile(invalidJSONPath, []byte("{invalid json}"), 0644)
		if err != nil {
			t.Fatalf("Failed to create invalid JSON file: %v", err)
		}

		_, err = config.GetCppgCfgFromPath(invalidJSONPath)
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
	})
}
