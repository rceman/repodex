package scan

import (
	"strings"

	"github.com/memkit/repodex/internal/profile"
)

type pattern struct {
	value   string
	negate  bool
	dirOnly bool
}

type ignoreMatcher struct {
	patterns []pattern
}

func newIgnoreMatcher(patterns []string) ignoreMatcher {
	var compiled []pattern
	for _, raw := range patterns {
		if raw == "" {
			continue
		}
		negate := strings.HasPrefix(raw, "!")
		value := strings.TrimPrefix(raw, "!")
		dirOnly := strings.HasSuffix(value, "/")
		normalized := strings.TrimSuffix(value, "/")
		if dirOnly {
			if !strings.HasPrefix(normalized, "**/") {
				normalized = "**/" + normalized
			}
			if !strings.HasSuffix(normalized, "/**") {
				normalized = normalized + "/**"
			}
		}
		compiled = append(compiled, pattern{
			value:   normalized,
			negate:  negate,
			dirOnly: dirOnly,
		})
	}
	return ignoreMatcher{patterns: compiled}
}

func (m ignoreMatcher) shouldIgnore(path string, isDir bool) bool {
	ignored := false
	for _, p := range m.patterns {
		if p.matches(path, isDir) {
			ignored = !p.negate
		}
	}
	return ignored
}

func (p pattern) matches(path string, isDir bool) bool {
	match, err := profile.GlobMatch(p.value, path)
	if err == nil && match {
		return true
	}
	if p.dirOnly {
		return isDir && path == p.value
	}
	return false
}
