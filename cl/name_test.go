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
