package search

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEnrichGoScopes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	writeFile(t, path, []byte(strings.Join([]string{
		"package main",
		"",
		"func first() {",
		"    call()",
		"}",
		"",
		"func second() {",
		"    first()",
		"}",
	}, "\n")))

	results := []Result{
		{Path: "sample.go", StartLine: 3, MatchLine: 4},
		{Path: "sample.go", StartLine: 7, MatchLine: 8},
		{Path: "sample.go", StartLine: 1, MatchLine: 1},
	}

	enrichGoScopes(root, results)

	first := results[0]
	if first.ScopeStartLine != 3 || first.ScopeEndLine != 5 {
		t.Fatalf("expected first scope 3-5, got %d-%d", first.ScopeStartLine, first.ScopeEndLine)
	}
	if first.ScopeKind != "func" || first.ScopeName != "first" {
		t.Fatalf("unexpected first scope metadata: kind=%q name=%q", first.ScopeKind, first.ScopeName)
	}

	second := results[1]
	if second.ScopeStartLine != 7 || second.ScopeEndLine != 9 {
		t.Fatalf("expected second scope 7-9, got %d-%d", second.ScopeStartLine, second.ScopeEndLine)
	}
	if second.ScopeKind != "func" || second.ScopeName != "second" {
		t.Fatalf("unexpected second scope metadata: kind=%q name=%q", second.ScopeKind, second.ScopeName)
	}

	none := results[2]
	if none.ScopeStartLine != 0 || none.ScopeEndLine != 0 || none.ScopeKind != "" || none.ScopeName != "" {
		t.Fatalf("expected empty scope for package line, got %d-%d kind=%q name=%q", none.ScopeStartLine, none.ScopeEndLine, none.ScopeKind, none.ScopeName)
	}
}
