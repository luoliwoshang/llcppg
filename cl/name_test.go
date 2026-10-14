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

package cl

import "testing"

func TestExternName(t *testing.T) {
	for _, tt := range []struct{ cName, goName string }{
		{"", ""},
		{"size_t", "SizeT"},
		{"time_t", "TimeT"},
		{"b_widget_t", "BWidgetT"},
		{"CXString", "CXString"},
		{"Foo_Bar", "Foo_Bar"},
		{"CODE_MASK", "CODE_MASK"},
		{"foo__bar", "Foo_Bar"},
		{"foo_", "Foo_"},
		{"_foo_bar", "X_fooBar"},
		{"__foo", "X__Foo"},
		{"_", "X_"},
		{"bar::base", "BarBase"},
		{"foo_URL_ID", "FooURL_ID"},
	} {
		t.Run(tt.cName, func(t *testing.T) {
			if got := externName(tt.cName); got != tt.goName {
				t.Errorf("externName(%q) = %q, want %q", tt.cName, got, tt.goName)
			}
		})
	}
}

func TestLocalNamingConfiguration(t *testing.T) {
	p := &pkgCtx{
		nsPrefix:       []string{"ns::"},
		typePrefix:     []string{"local_"},
		typeSuffix:     []string{"_t"},
		rename:         map[string]string{"special_t": "Special", "word": "Term"},
		forceCamelCase: true,
	}
	for _, tt := range []struct{ cName, goName string }{
		{"ns::local_widget_t", "Widget"},
		{"special_t", "Special"},
		{"local_word_t", "Term"},
		{"Foo_Bar", "FooBar"},
	} {
		if got := p.typeName(tt.cName, true); got != tt.goName {
			t.Errorf("typeName(%q) = %q, want %q", tt.cName, got, tt.goName)
		}
	}
	if got := p.localName("word_value", false); got != "TermValue" {
		t.Errorf("localName = %q, want TermValue", got)
	}
}

func TestFuncNameTypeAbbr(t *testing.T) {
	for _, tt := range []struct {
		name, cName, typName, rule, want string
	}{
		{"lowercase", "gzclearerr", "GzFile", "GzFile: Gz", "Clearerr"},
		{"read", "gzread", "GzFile", "GzFile: Gz", "Read"},
		{"mixed_case", "gzclose_r", "GzFile", "GzFile: Gz", "CloseR"},
		{"uppercase", "gzRead", "GzFile", "GzFile: Gz", "Read"},
		{"after_get", "getGzclearerr", "GzFile", "GzFile: Gz", "Clearerr"},
		{"before_get", "gzGetRead", "GzFile", "GzFile: Gz", "Read"},
		{"keep_lowercase_get", "gzgetread", "GzFile", "GzFile: Gz", "Getread"},
		{"whole_name", "gz", "GzFile", "GzFile: Gz", "Gz"},
		{"digit_after_prefix", "gz2read", "GzFile", "GzFile: Gz", "Gz2read"},
		{"case_sensitive", "gzread", "GzFile", "GzFile: gz", "Gzread"},
		{"suffix", "getExpansionLocation", "SourceLocation", "SourceLocation: Location", "Expansion"},
		{"wildcard", "gzread", "GzFile", "*File: $1", "Read"},
		{"multiple_abbrs", "gzread", "GzFile", "GzFile: Gzip, Gz", "Read"},
		{"unmatched_type", "gzread", "GzFile", "SourceLocation: Location", "Gzread"},
		{"default_lowercase", "gzread", "Gz", "", "Gzread"},
		{"default_uppercase", "gzRead", "Gz", "", "Read"},
		{"default_after_get", "getGzclearerr", "Gz", "", "Gzclearerr"},
		{"default_keep_lowercase_get", "gzgetread", "Gz", "", "Gzgetread"},
		{"default_whole_name", "gz", "Gz", "", "Gz"},
		{"default_digit_after_prefix", "gz2read", "Gz", "", "Gz2read"},
		{"unmatched_rule_default_prefix", "gzread", "Gz", "SourceLocation: Location", "Gzread"},
		{"typedef_name", "getTypedefName", "Type", "", "TypedefName"},
		{"explicit_same_name", "gzread", "Gz", "Gz: Gz", "Read"},
		{"global_function", "gzread", "", "GzFile: Gz", "Gzread"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var rules []string
			if tt.rule != "" {
				rules = []string{tt.rule}
			}
			abbr, err := newMatcher("TypeAbbr", rules)
			if err != nil {
				t.Fatal(err)
			}
			p := &pkgCtx{typeAbbr: abbr}
			if got := p.funcName(tt.cName, -1, tt.typName, "", true, true); got != tt.want {
				t.Errorf("funcName(%q, type %q) = %q, want %q", tt.cName, tt.typName, got, tt.want)
			}
		})
	}
}
