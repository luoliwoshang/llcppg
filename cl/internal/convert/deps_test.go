package convert

import (
	"reflect"
	"testing"
)

func TestListPatterns_NormalizeAndDedup(t *testing.T) {
	deps := []string{
		"c",
		"github.com/goplus/lib/c",
		"github.com/goplus/lib/c@v0.3.1",
		"github.com/goplus/llpkg/libxml2@v1.0.1",
		"github.com/goplus/llpkg/libxml2",
	}
	got := listPatterns(deps)
	want := []string{
		"github.com/goplus/lib/c",
		"github.com/goplus/llpkg/libxml2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listPatterns() mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestListPatterns_EmptyDepsFallbackAll(t *testing.T) {
	got := listPatterns(nil)
	want := []string{"all"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listPatterns() mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}
}
