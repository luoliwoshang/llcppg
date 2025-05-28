package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/goplus/llcppg/config"
	"github.com/goplus/llpkgstore/upstream"
	"github.com/goplus/llpkgstore/upstream/installer/conan"
)

const llcppgGoVersion = "1.20.14"

type testCase struct {
	modpath string
	dir     string
	pkg     upstream.Package
	config  map[string]string // conan options
}

var testCases = []testCase{
	{
		modpath: "github.com/goplus/llcppg/_cmptest/testdata/cjson/1.7.18/cjson",
		dir:     "./testdata/cjson/1.7.18",
		pkg:     upstream.Package{Name: "cjson", Version: "1.7.18"},
		config: map[string]string{
			"options": "utils=True",
		},
	},
}

func TestEnd2End(t *testing.T) {
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.pkg.Name, func(t *testing.T) {
			t.Parallel()
			testFrom(t, tc, false)
		})
	}
}

func testFrom(t *testing.T, tc testCase, gen bool) {
	wd, _ := os.Getwd()
	conanDir, err := os.MkdirTemp("", "llcppg_end2end_test_conan_dir_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(conanDir)

	resultDir, err := os.MkdirTemp("", "llcppg_end2end_test_result_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(resultDir)

	cfgPath := filepath.Join(wd, tc.dir, config.LLCPPG_CFG)
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(resultDir, config.LLCPPG_CFG), cfg, os.ModePerm)
	_, err = conan.NewConanInstaller(tc.config).Install(tc.pkg, conanDir)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("llcppg", "-v", "-mod="+tc.modpath)
	cmd.Dir = resultDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	lockGoVersion(cmd, conanDir)

	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	// llcppg.symb.json is a middle file
	os.Remove(filepath.Join(resultDir, config.LLCPPG_SYMB))

	if gen {
		os.RemoveAll(filepath.Join(wd, tc.dir))
		os.Rename(resultDir, filepath.Join(wd, tc.dir))
		return
	}

	diffCmd := exec.Command("git", "diff", "--no-index", tc.dir, resultDir)
	diffCmd.Dir = wd
	diffCmd.Stdout = os.Stdout
	diffCmd.Stderr = os.Stderr
	err = diffCmd.Run()
	if err != nil {
		t.Fatal(err)
	}
}

func appendPCPath(path string) string {
	if env, ok := os.LookupEnv("PKG_CONFIG_PATH"); ok {
		return path + ":" + env
	}
	return path
}

// lockGoVersion locks current Go version to `llcppgGoVersion` via GOTOOLCHAIN
func lockGoVersion(cmd *exec.Cmd, pcPath string) {
	// don't change global settings, use temporary environment.
	// see issue: https://github.com/goplus/llpkgstore/issues/18
	setPath(cmd, pcPath)
	cmd.Env = append(cmd.Env, fmt.Sprintf("GOTOOLCHAIN=go%s", llcppgGoVersion))
}

func setPath(cmd *exec.Cmd, path string) {
	pcPath := fmt.Sprintf("PKG_CONFIG_PATH=%s", appendPCPath(path))
	cmd.Env = append(os.Environ(), pcPath)
}
