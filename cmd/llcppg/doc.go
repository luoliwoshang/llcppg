/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/goplus/llcppg/internal/docserver"
)

// runDoc serves the documentation of the Go package in the current directory
// (`llcppg -doc`). It blocks until interrupted (SIGINT), then shuts down
// gracefully. Usage errors (no Go files, more than one package, a bad listen
// address) are reported and exit non-zero.
func runDoc() {
	// The -doc-only flags are meaningless without -doc; using one on its own is
	// a usage error rather than a silent no-op.
	if err := docOnlyFlagError(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if args := flag.Args(); len(args) > 0 {
		fmt.Fprintln(os.Stderr, "llcppg: -doc takes no arguments; run it in the directory of the package to document")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := docserver.Run(ctx, ".", docserver.Options{
		Addr:        *docHTTP,
		OpenBrowser: *docOpen,
		AllDecls:    *docAll,
		AutoExit:    *docAutoExit,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// docFlags are the flags that only make sense together with -doc. Passing any
// of them without -doc is a usage error (see docOnlyFlagError).
var docFlags = []string{"autoexit", "open", "all", "http"}

// docOnlyFlagError returns a usage error when a -doc-only flag was passed
// without -doc, and nil otherwise. passed reports whether a given flag was set
// on the command line. It is separated from main so it can be unit-tested
// without flag parsing or os.Exit; -autoexit defaults to true, so the presence
// of the flag (not its value) is what distinguishes a real request from the
// default.
func docOnlyFlagError() error {
	for _, name := range docFlags {
		if flagPassed(name) {
			return fmt.Errorf("llcppg: -%s is only valid with -doc", name)
		}
	}
	return nil
}

// flagPassed reports whether the named flag was set on the command line (as
// opposed to left at its default). It lets us reject -doc-only flags such as
// -autoexit when they are used without -doc, even though their default makes
// the flag value alone ambiguous.
func flagPassed(name string) bool {
	passed := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			passed = true
		}
	})
	return passed
}
