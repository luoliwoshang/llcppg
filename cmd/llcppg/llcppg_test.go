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
	"strings"
	"testing"
)

// TestDocOnlyFlagError checks the rule that -doc-only flags (such as -autoexit)
// are usage errors when passed without -doc, and are accepted with -doc.
func TestDocOnlyFlagError(t *testing.T) {
	passedSet := func(names ...string) func(string) bool {
		set := make(map[string]bool, len(names))
		for _, n := range names {
			set[n] = true
		}
		return func(name string) bool { return set[name] }
	}

	tests := []struct {
		name    string
		docMode bool
		passed  func(string) bool
		wantErr string // substring, empty means no error
	}{
		{"no flags, no doc", false, passedSet(), ""},
		{"autoexit without doc", false, passedSet("autoexit"), "-autoexit is only valid with -doc"},
		{"open without doc", false, passedSet("open"), "-open is only valid with -doc"},
		{"all without doc", false, passedSet("all"), "-all is only valid with -doc"},
		{"http without doc", false, passedSet("http"), "-http is only valid with -doc"},
		{"autoexit with doc", true, passedSet("autoexit"), ""},
		{"all doc flags with doc", true, passedSet("autoexit", "open", "all", "http"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := docOnlyFlagError(tt.docMode, tt.passed)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("docOnlyFlagError = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("docOnlyFlagError = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

// TestAutoExitFlagParsing confirms -autoexit defaults to true and parses
// -autoexit=false, matching the documented behavior. It uses a local FlagSet so
// it does not disturb the package's global flags.
func TestAutoExitFlagParsing(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"default is on", []string{"-doc"}, true},
		{"explicitly off", []string{"-doc", "-autoexit=false"}, false},
		{"explicitly on", []string{"-doc", "-autoexit=true"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("llcppg", flag.ContinueOnError)
			fs.Bool("doc", false, "")
			autoExit := fs.Bool("autoexit", true, "")
			if err := fs.Parse(tt.args); err != nil {
				t.Fatalf("Parse(%v) error: %v", tt.args, err)
			}
			if *autoExit != tt.want {
				t.Errorf("autoexit = %v, want %v", *autoExit, tt.want)
			}
		})
	}
}
