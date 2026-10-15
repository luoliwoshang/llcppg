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
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/llcppg/tool/initpkg"
)

func runInit() {
	// The module name is optional: `llcppg -init <module-name>` uses the
	// given name, `llcppg -init` infers it from the current directory.
	module := ""
	if args := flag.Args(); len(args) > 0 {
		module = args[0]
	}
	dir, err := cacheDir()
	check(err)
	if err := initpkg.Init(".", module, &initpkg.Config{CacheDir: dir}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// CacheDir returns the directory llcppg uses to cache downloadable resources
// such as the project template. The LLCPPG_CACHE environment variable overrides
// the default, which is <os.UserCacheDir>/llcppg.
func cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate user cache dir: %w", err)
	}
	return filepath.Join(base, "llcppg"), nil
}
