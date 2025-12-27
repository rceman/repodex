package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/hash"
)

// BuildEffectiveRules merges profile rules and user overrides.
func BuildEffectiveRules(root string, profiles []string, cfg config.Config) (EffectiveRules, error) {
	resolved, err := ResolveProfiles(profiles)
	if err != nil {
		return EffectiveRules{}, err
	}

	scanIgnore, err := loadScanIgnore(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return EffectiveRules{}, fmt.Errorf("load .repodexignore: %w", err)
	}
	if errors.Is(err, os.ErrNotExist) {
		scanIgnore = buildDefaultIgnore(resolved)
	}

	includeExt := mergeIncludeExt(resolved)
	if len(includeExt) == 0 {
		return EffectiveRules{}, fmt.Errorf("profiles do not define any include extensions")
	}
	tokenRules := mergeTokenRules(cfg.Token, resolved)
	tokenCfg := tokenRules.ToConfig(cfg.Token)
	scanSettings := ScanSettings{
		MaxTextFileSizeBytes: cfg.Scan.MaxTextFileSizeBytes,
	}

	rulesHash, err := computeRulesHash(profiles, scanIgnore, includeExt, scanSettings, tokenRules)
	if err != nil {
		return EffectiveRules{}, err
	}

	return EffectiveRules{
		ScanIgnore:   scanIgnore,
		IncludeExt:   includeExt,
		Tokenize:     tokenRules,
		TokenConfig:  tokenCfg,
		Profiles:     append([]string(nil), profiles...),
		ScanSettings: scanSettings,
		RulesHash:    rulesHash,
	}, nil
}

func mergeTokenRules(base config.TokenizationConfig, profiles []Profile) TokenizeRules {
	eff := TokenizeRules{
		PathStripSuffixes: append([]string{}, base.PathStripSuffixes...),
		PathStripExts:     append([]string{}, base.PathStripExts...),
		StopWords:         append([]string{}, base.StopWords...),
		AllowShortTokens:  append([]string{}, base.AllowShortTokens...),
		MinTokenLen:       base.MinTokenLen,
		MaxTokenLen:       base.MaxTokenLen,
		DropHexLen:        base.DropHexLen,
	}
	tokenizeStrings := base.TokenizeStringLiterals
	eff.TokenizeStrings = &tokenizeStrings

	for _, p := range profiles {
		r := p.Rules().Tokenize
		if len(r.PathStripSuffixes) > 0 {
			eff.PathStripSuffixes = append(eff.PathStripSuffixes, r.PathStripSuffixes...)
		}
		if len(r.PathStripExts) > 0 {
			eff.PathStripExts = append(eff.PathStripExts, r.PathStripExts...)
		}
		if len(r.StopWords) > 0 {
			eff.StopWords = append(eff.StopWords, r.StopWords...)
		}
		if len(r.AllowShortTokens) > 0 {
			eff.AllowShortTokens = append(eff.AllowShortTokens, r.AllowShortTokens...)
		}
		if r.MinTokenLen > 0 {
			eff.MinTokenLen = r.MinTokenLen
		}
		if r.MaxTokenLen > 0 {
			eff.MaxTokenLen = r.MaxTokenLen
		}
		if r.DropHexLen > 0 {
			eff.DropHexLen = r.DropHexLen
		}
		if r.TokenizeStrings != nil {
			eff.TokenizeStrings = r.TokenizeStrings
		}
	}
	return eff
}

func mergeIncludeExt(profiles []Profile) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, p := range profiles {
		r := p.Rules()
		for _, ext := range r.IncludeExt {
			normalized := strings.ToLower(strings.TrimSpace(ext))
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			out = append(out, normalized)
		}
	}
	return out
}

func buildDefaultIgnore(profiles []Profile) []string {
	patterns := append([]string{}, GlobalScanIgnore()...)
	for _, p := range profiles {
		if rules := p.Rules(); len(rules.ScanIgnore) > 0 {
			patterns = append(patterns, rules.ScanIgnore...)
		}
	}
	return patterns
}

// ToConfig converts tokenization rules into a TokenizationConfig using base defaults.
func (t TokenizeRules) ToConfig(base config.TokenizationConfig) config.TokenizationConfig {
	out := base
	out.PathStripSuffixes = append([]string{}, t.PathStripSuffixes...)
	out.PathStripExts = append([]string{}, t.PathStripExts...)
	out.StopWords = append([]string{}, t.StopWords...)
	out.AllowShortTokens = append([]string{}, t.AllowShortTokens...)
	if t.MinTokenLen > 0 {
		out.MinTokenLen = t.MinTokenLen
	}
	if t.MaxTokenLen > 0 {
		out.MaxTokenLen = t.MaxTokenLen
	}
	if t.DropHexLen > 0 {
		out.DropHexLen = t.DropHexLen
	}
	if t.TokenizeStrings != nil {
		out.TokenizeStringLiterals = *t.TokenizeStrings
	}
	return out
}

func loadScanIgnore(root string) ([]string, error) {
	path := filepath.Join(root, ".repodexignore")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var patterns []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		patterns = append(patterns, filepath.ToSlash(trimmed))
	}
	return patterns, nil
}

func computeRulesHash(profiles []string, scanIgnore []string, includeExt []string, scanSettings ScanSettings, tokenize TokenizeRules) (uint64, error) {
	state := struct {
		SchemaVersion int
		Profiles      []string
		ScanIgnore    []string
		IncludeExt    []string
		ScanSettings  ScanSettings
		Tokenize      TokenizeRules
	}{
		SchemaVersion: SchemaVersion,
		Profiles:      append([]string(nil), profiles...),
		ScanIgnore:    append([]string(nil), scanIgnore...),
		IncludeExt:    append([]string(nil), includeExt...),
		ScanSettings:  scanSettings,
		Tokenize:      tokenize,
	}

	bytes, err := json.Marshal(state)
	if err != nil {
		return 0, err
	}
	return hash.Sum64(bytes), nil
}

// GlobMatch wraps doublestar for deterministic behavior.
func GlobMatch(pattern, path string) (bool, error) {
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	match := matchSegments(patternParts, pathParts)
	return match, nil
}

func matchSegments(patternParts, pathParts []string) bool {
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}
	head := patternParts[0]
	if head == "**" {
		for i := 0; i <= len(pathParts); i++ {
			if matchSegments(patternParts[1:], pathParts[i:]) {
				return true
			}
		}
		return false
	}
	if len(pathParts) == 0 {
		return false
	}
	ok, err := filepath.Match(head, pathParts[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(patternParts[1:], pathParts[1:])
}
