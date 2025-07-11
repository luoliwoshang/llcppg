package parse

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/goplus/llcppg/_xtool/internal/clangtool"
	"github.com/goplus/llcppg/_xtool/internal/header"
	"github.com/goplus/llcppg/_xtool/internal/parser"
	llcppg "github.com/goplus/llcppg/config"
	"github.com/goplus/llgo/xtool/clang/preprocessor"
)

type dbgFlags = int

var debugParse bool

const (
	DbgParse   dbgFlags = 1 << iota
	DbgFlagAll          = DbgParse
)

func SetDebug(dbgFlags dbgFlags) {
	debugParse = (dbgFlags & DbgParse) != 0
}

type Config struct {
	Conf   *llcppg.Config
	Out    bool     // if gen llcppg.sigfetch.json
	Cflags []string // other cflags want to parse

	ExtractMode bool
	ExtractFile string
	IsTemp      bool
	IsCpp       bool

	CombinedFile     string
	PreprocessedFile string
	Exec             func(conf *Config, pkg *llcppg.Pkg)
}

func Do(conf *Config) error {
	if debugParse {
		parser.SetDebug(parser.DbgFlagAll)
		fmt.Fprintln(os.Stderr, "output to file:", conf.Out)
		if conf.ExtractMode {
			fmt.Fprintln(os.Stderr, "runExtract: extractFile:", conf.ExtractFile)
			fmt.Fprintln(os.Stderr, "isTemp:", conf.IsTemp)
			fmt.Fprintln(os.Stderr, "isCpp:", conf.IsCpp)
			fmt.Fprintln(os.Stderr, "out:", conf.Out)
			fmt.Fprintln(os.Stderr, "otherArgs:", conf.Cflags)
		}
	}

	var isCpp bool
	if conf.IsCpp {
		isCpp = true
	} else {
		isCpp = conf.Conf.Cplusplus
	}
	if err := createTempIfNoExist(&conf.CombinedFile, conf.Conf.Name+"*.h"); err != nil {
		return err
	}
	if err := createTempIfNoExist(&conf.PreprocessedFile, conf.Conf.Name+"*.i"); err != nil {
		return err
	}

	if debugParse {
		fmt.Fprintln(os.Stderr, "Do: combinedFile", conf.CombinedFile)
		fmt.Fprintln(os.Stderr, "Do: preprocessedFile", conf.PreprocessedFile)
	}

	// compose includes to a combined file
	err := clangtool.ComposeIncludes(conf.Conf.Include, conf.CombinedFile)
	if err != nil {
		return err
	}

	// prepare clang flags to preprocess the combined file
	var clangFlags []string

	if env := os.Getenv("TARGET"); env != "" {
		clangFlags = append(clangFlags, strings.Fields(env)...)
	}

	clangFlags = append(clangFlags, strings.Fields(conf.Conf.CFlags)...)
	if !isCpp {
		clangFlags = append(clangFlags, "-x", "c")
	} else {
		clangFlags = append(clangFlags, "-x", "c++")
	}
	clangFlags = append(clangFlags, "-C")  // keep comment
	clangFlags = append(clangFlags, "-dD") // keep macro
	clangFlags = append(clangFlags, "-fparse-all-comments")

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	ppconf := &preprocessor.Config{
		Compiler: "clang",
		Flags:    clangFlags,
		BaseDir:  pwd,
	}

	fmt.Fprintln(os.Stderr, "pwd", pwd)
	fmt.Fprintln(os.Stderr, "clangFlags", clangFlags)
	fmt.Fprintln(os.Stderr, "conf.CombinedFile", conf.CombinedFile)
	fmt.Fprintln(os.Stderr, "conf.PreprocessedFile", conf.PreprocessedFile)

	err = preprocessor.Do(conf.CombinedFile, conf.PreprocessedFile, ppconf)
	if err != nil {
		return err
	}

	// https://github.com/goplus/llgo/issues/603
	// we need exec.Command("clang", "-print-resource-dir").Output() in llcppsigfetch to obtain the resource directory
	// to ensure consistency between clang preprocessing and libclang-extracted header filelink cflags.
	// Currently, directly calling exec.Command in the main flow of llcppsigfetch will cause hang and fail to execute correctly.
	// As a solution, the resource directory is externally provided by llcppg.
	libclangFlags := []string{"-fparse-all-comments"}

	pkgHfiles := header.PkgHfileInfo(&header.Config{
		Includes: conf.Conf.Include,
		Args:     append(libclangFlags, strings.Fields(conf.Conf.CFlags)...),
		Mix:      conf.Conf.Mix,
	})
	if debugParse {
		fmt.Fprintln(os.Stderr, "interfaces", pkgHfiles.Inters)
		fmt.Fprintln(os.Stderr, "implements", pkgHfiles.Impls)
		fmt.Fprintln(os.Stderr, "thirdhfile", pkgHfiles.Thirds)
	}
	libclangFlags = append(libclangFlags, strings.Fields(conf.Conf.CFlags)...)
	file, err := parser.Do(&parser.ConverterConfig{
		File:  conf.PreprocessedFile,
		Args:  libclangFlags,
		IsCpp: isCpp,
	})
	if err != nil {
		return err
	}

	pkg := &llcppg.Pkg{
		File:    file,
		FileMap: make(map[string]*llcppg.FileInfo),
	}

	fileTypeMappings := []struct {
		files    []string
		fileType llcppg.FileType
	}{
		{pkgHfiles.Inters, llcppg.Inter},
		{pkgHfiles.Impls, llcppg.Impl},
		{pkgHfiles.Thirds, llcppg.Third},
	}

	for _, mapping := range fileTypeMappings {
		for _, file := range mapping.files {
			pkg.FileMap[file] = &llcppg.FileInfo{
				FileType: mapping.fileType,
			}
		}
	}

	if debugParse {
		fmt.Fprintln(os.Stderr, "Have %d Macros", len(pkg.File.Macros))
		for _, macro := range pkg.File.Macros {
			fmt.Fprintf(os.Stderr, "Macro %s", macro.Name)
		}
		fmt.Fprintln(os.Stderr)
	}

	if conf.Exec != nil {
		conf.Exec(conf, pkg)
	}

	return nil
}

func OutputPkg(conf *Config, pkg *llcppg.Pkg) {
	info := MarshalPkg(pkg)
	str, _ := json.MarshalIndent(&info, "", "  ")
	outputResult(str, conf.Out)
}

func outputResult(result []byte, outputToFile bool) {
	if outputToFile {
		outputFile := llcppg.LLCPPG_SIGFETCH
		err := os.WriteFile(outputFile, result, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Results saved to %s\n", outputFile)
	} else {
		fmt.Println(string(result))
	}
}

func createTempIfNoExist(filename *string, pattern string) error {
	if *filename != "" {
		return nil
	}
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return err
	}
	*filename = f.Name()
	return nil
}
