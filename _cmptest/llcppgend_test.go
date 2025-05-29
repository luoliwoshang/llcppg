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
	modpath  string
	dir      string
	pkg      upstream.Package
	config   map[string]string // conan options
	demosDir string
}

var testCases = []testCase{
	{
		modpath: "github.com/goplus/llcppg/_cmptest/testdata/cjson/1.7.18/cjson",
		dir:     "./testdata/cjson/1.7.18",
		pkg:     upstream.Package{Name: "cjson", Version: "1.7.18"},
		config: map[string]string{
			"options": "utils=True",
		},
		demosDir: "./testdata/cjson/demo",
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
	dir := filepath.Join(wd, tc.dir)
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

	cmd := command("llcppg", resultDir, "-v", "-mod="+tc.modpath)
	lockGoVersion(cmd, conanDir)

	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	// llcppg.symb.json is a middle file
	os.Remove(filepath.Join(resultDir, config.LLCPPG_SYMB))

	if gen {
		os.RemoveAll(dir)
		os.Rename(resultDir, dir)
	} else {
		// check the result is the same as the expected result
		diffCmd := exec.Command("git", "diff", "--no-index", dir, resultDir)
		diffCmd.Dir = wd
		diffCmd.Stdout = os.Stdout
		diffCmd.Stderr = os.Stderr
		err = diffCmd.Run()
		if err != nil {
			t.Fatal(err)
		}
	}
	runDemos(t, filepath.Join(wd, tc.demosDir), tc.pkg.Name, filepath.Join(dir, tc.pkg.Name))
}

// pkgpath is the filepath use to replace the import path in demo's go.mod
func runDemos(t *testing.T, demosPath string, pkgname, pkgpath string) {
	goMod := command("go", demosPath, "mod", "init", "test")
	err := goMod.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(filepath.Join(demosPath, "go.mod"))

	replace := command("go", demosPath, "mod", "edit", "-replace", pkgname+"="+pkgpath)
	err = replace.Run()
	if err != nil {
		t.Fatal(err)
	}

	tidy := command("go", demosPath, "mod", "tidy")
	err = tidy.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(filepath.Join(demosPath, "go.sum"))

	demos, err := os.ReadDir(demosPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, demo := range demos {
		if !demo.IsDir() {
			continue
		}
		demoPath := filepath.Join(demosPath, demo.Name())
		demoCmd := command("llgo", demosPath, "run", demoPath)
		err = demoCmd.Run()
		if err != nil {
			t.Fatal(err)
		}
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

func command(app string, dir string, args ...string) *exec.Cmd {
	cmd := exec.Command(app, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
