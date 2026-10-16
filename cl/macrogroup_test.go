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

package cl_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goplus/llcppg/cl"
	"github.com/goplus/llcppg/cl/cltest"
	"github.com/goplus/llcppg/clang"
)

// parseMacroGroupTU writes src to a temp header, parses it, and returns the
// translation unit together with its header path.
func parseMacroGroupTU(t *testing.T, idx clang.Index, src string) (clang.TranslationUnit, string) {
	t.Helper()
	dir := t.TempDir()
	hfile := filepath.Join(dir, "in.h")
	if err := os.WriteFile(hfile, []byte(src), 0644); err != nil {
		t.Fatal("write header:", err)
	}
	u, err := idx.ParseTranslationUnit(
		clang.DetailedPreprocessingRecord, hfile, "-x", cltest.LanguageOf(cl.LanguageC), "-fparse-all-comments")
	if err != nil {
		t.Fatal("ParseTranslationUnit:", err)
	}
	return u, hfile
}

func newMacroGroupPackage(t *testing.T, src string, group map[string]string) error {
	t.Helper()
	idx := clang.CreateIndex(1, 1)
	defer idx.Dispose()

	u, hfile := parseMacroGroupTU(t, idx, src)
	defer u.Dispose()

	_, err := cl.NewPackage("testcl/foo", "foo", []cl.Source{u}, &cl.Config{
		Language:      cl.LanguageC,
		MacroGroup:    group,
		FailFast:      0,
		PubFileLookup: func(pkgPath string) (string, bool) { return "", false },
		// Restrict processing to the test header so compiler built-in macros
		// (which have no bearing on the test) are not pulled into groups.
		PackageOf: func(headerFile string) (string, bool) {
			if headerFile == hfile {
				return "testcl/foo", true
			}
			return "", false
		},
	})
	return err
}

// TestMacroGroupErrors covers the configuration-error cases: each must make
// cl.NewPackage return a non-nil error so conversion stops rather than silently
// producing a different grouping than the user asked for.
func TestMacroGroupErrors(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		group map[string]string
	}{
		{
			name:  "unknown first macro",
			src:   "#define A 1\n#define B 2\n",
			group: map[string]string{"NOPE": "B"},
		},
		{
			name:  "unknown last macro",
			src:   "#define A 1\n#define B 2\n",
			group: map[string]string{"A": "NOPE"},
		},
		{
			name:  "reversed range (last before first)",
			src:   "#define A 1\n#define B 2\n#define C 3\n",
			group: map[string]string{"C": "A"},
		},
		{
			name:  "non-macro declaration inside range",
			src:   "#define A 1\nint x;\n#define B 2\n",
			group: map[string]string{"A": "B"},
		},
		{
			name:  "overlapping ranges",
			src:   "#define A 1\n#define B 2\n#define C 3\n#define D 4\n",
			group: map[string]string{"A": "C", "B": "D"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := newMacroGroupPackage(t, tt.src, tt.group); err == nil {
				t.Errorf("expected an error for %q, got nil", tt.name)
			}
		})
	}
}

// TestMacroGroupValid confirms a well-formed configured range and a plain
// automatic run both succeed.
func TestMacroGroupValid(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		group map[string]string
	}{
		{
			name:  "configured range across a blank line",
			src:   "#define A 1\n\n#define B 2\n#define C 3\n",
			group: map[string]string{"A": "C"},
		},
		{
			name:  "automatic run with no config",
			src:   "#define A 1\n#define B 2\n#define C 3\n",
			group: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := newMacroGroupPackage(t, tt.src, tt.group); err != nil {
				t.Errorf("unexpected error for %q: %v", tt.name, err)
			}
		})
	}
}
