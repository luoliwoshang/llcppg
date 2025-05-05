/*
 * Copyright (c) 2024 The GoPlus Authors (goplus.org). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/goplus/llcppg/_xtool/llcppsymg/symg"
	"github.com/goplus/llcppg/_xtool/llcppsymg/tool/arg"
	"github.com/goplus/llcppg/_xtool/llcppsymg/tool/config"
	llcppg "github.com/goplus/llcppg/config"
)

func main() {
	args, _ := arg.ParseArgs(os.Args[1:], llcppg.LLCPPG_CFG, nil)

	if args.Help {
		printUsage()
		return
	}

	var data []byte
	var err error
	if args.UseStdin {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(args.CfgFile)
	}

	check(err)
	conf, err := config.GetConf(data)
	check(err)
	defer conf.Delete()

	if args.VerboseParseIsMethod {
		symg.SetDebug(symg.DbgParseIsMethod)
	}

	if args.Verbose {
		symg.SetDebug(symg.DbgSymbol)
		if args.UseStdin {
			fmt.Println("Config From Stdin")
		} else {
			fmt.Println("Config From File", args.CfgFile)
		}
		fmt.Println("Name:", conf.Name)
		fmt.Println("CFlags:", conf.CFlags)
		fmt.Println("Libs:", conf.Libs)
		fmt.Println("Include:", conf.Include)
		fmt.Println("TrimPrefixes:", conf.TrimPrefixes)
		fmt.Println("Cplusplus:", conf.Cplusplus)
		fmt.Println("SymMap:", conf.SymMap)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to parse config file:", args.CfgFile)
	}
	symbols, err := symg.ParseDylibSymbols(conf.Libs)
	check(err)

	pkgHfiles := config.PkgHfileInfo(conf.Config, []string{})
	if args.Verbose {
		fmt.Println("interfaces", pkgHfiles.Inters)
		fmt.Println("implements", pkgHfiles.Impls)
		fmt.Println("thirdhfile", pkgHfiles.Thirds)
	}
	headerInfos, err := symg.ParseHeaderFile(pkgHfiles.CurPkgFiles(), conf.TrimPrefixes, strings.Fields(conf.CFlags), conf.SymMap, conf.Cplusplus, false)
	check(err)

	symbolData, err := symg.GenerateSymTable(symbols, headerInfos)
	check(err)

	err = os.WriteFile(llcppg.LLCPPG_SYMB, symbolData, 0644)
	check(err)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: llcppsymg [-v] [config-file]")
}
