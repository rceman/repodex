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

// BuildEffectiveRules merges global defaults, profile rules, and user overrides.
func BuildEffectiveRules(root string, cfg config.Config) (EffectiveRules, error) {
	ctx := DetectContext{Root: root}
	detected, err := DetectProfiles(ctx)
	if err != nil {
		return EffectiveRules{}, err
	}

	scanIgnore := append([]string{}, GlobalScanIgnore(detected.HasPackageJSON)...)
	for _, p := range detected.Profiles {
		if rules := p.Rules(); len(rules.ScanIgnore) > 0 {
			scanIgnore = append(scanIgnore, rules.ScanIgnore...)
		}
	}
	if userPatterns, err := loadScanIgnore(root); err == nil {
		scanIgnore = append(scanIgnore, userPatterns...)
	} else if !errors.Is(err, os.ErrNotExist) {
		return EffectiveRules{}, fmt.Errorf("load .scanignore: %w", err)
	}

	tokenRules := mergeTokenRules(cfg.Token, detected.Profiles)
	userToken, err := loadTokenizeOverride(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return EffectiveRules{}, err
	}
	tokenRules = applyTokenizeOverride(tokenRules, userToken)
	tokenCfg := tokenRules.ToConfig(cfg.Token)

	detectedIDs := make([]string, 0, len(detected.Profiles))
	for _, p := range detected.Profiles {
		detectedIDs = append(detectedIDs, p.ID())
	}
	scanSettings := ScanSettings{
		MaxTextFileSizeBytes: cfg.Scan.MaxTextFileSizeBytes,
	}

	rulesHash, err := computeRulesHash(detectedIDs, scanIgnore, scanSettings, tokenRules)
	if err != nil {
		return EffectiveRules{}, err
	}

	return EffectiveRules{
		ScanIgnore:       scanIgnore,
		Tokenize:         tokenRules,
		TokenConfig:      tokenCfg,
		DetectedProfiles: detectedIDs,
		ScanSettings:     scanSettings,
		RulesHash:        rulesHash,
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

type listOverride struct {
	Mode   string   `json:"mode" yaml:"mode"`
	Values []string `json:"values" yaml:"values"`
}

type tokenizeOverride struct {
	PathStripSuffixes *listOverride `json:"path_strip_suffixes" yaml:"path_strip_suffixes"`
	PathStripExts     *listOverride `json:"path_strip_exts" yaml:"path_strip_exts"`
	StopWords         *listOverride `json:"stop_words" yaml:"stop_words"`
	AllowShortTokens  *listOverride `json:"allow_short_tokens" yaml:"allow_short_tokens"`
	MinTokenLen       *int          `json:"min_token_len" yaml:"min_token_len"`
	MaxTokenLen       *int          `json:"max_token_len" yaml:"max_token_len"`
	DropHexLen        *int          `json:"drop_hex_len" yaml:"drop_hex_len"`
	TokenizeStrings   *bool         `json:"tokenize_string_literals" yaml:"tokenize_string_literals"`
	AllowShortTokensB *bool         `json:"allow_short_tokens_enabled,omitempty" yaml:"allow_short_tokens_enabled,omitempty"` // reserved
}

func parseTokenizeOverride(data []byte) (tokenizeOverride, error) {
	var cfg tokenizeOverride
	if err := json.Unmarshal(data, &cfg); err != nil {
		return tokenizeOverride{}, fmt.Errorf("parse tokenize override: %w", err)
	}
	return cfg, nil
}

func loadTokenizeOverride(root string) (tokenizeOverride, error) {
	path := filepath.Join(root, ".repodex", "tokenize.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return tokenizeOverride{}, err
	}
	return parseTokenizeOverride(data)
}

func applyTokenizeOverride(base TokenizeRules, user tokenizeOverride) TokenizeRules {
	applyList := func(current []string, override *listOverride) []string {
		if override == nil {
			return current
		}
		mode := strings.ToLower(strings.TrimSpace(override.Mode))
		switch mode {
		case "replace":
			return append([]string{}, override.Values...)
		case "append", "":
			return append(current, override.Values...)
		default:
			return current
		}
	}
	base.PathStripSuffixes = applyList(base.PathStripSuffixes, user.PathStripSuffixes)
	base.PathStripExts = applyList(base.PathStripExts, user.PathStripExts)
	base.StopWords = applyList(base.StopWords, user.StopWords)
	base.AllowShortTokens = applyList(base.AllowShortTokens, user.AllowShortTokens)

	if user.MinTokenLen != nil && *user.MinTokenLen > 0 {
		base.MinTokenLen = *user.MinTokenLen
	}
	if user.MaxTokenLen != nil && *user.MaxTokenLen > 0 {
		base.MaxTokenLen = *user.MaxTokenLen
	}
	if user.DropHexLen != nil && *user.DropHexLen > 0 {
		base.DropHexLen = *user.DropHexLen
	}
	if user.TokenizeStrings != nil {
		base.TokenizeStrings = user.TokenizeStrings
	}
	return base
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
	path := filepath.Join(root, ".scanignore")
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

func computeRulesHash(profiles []string, scanIgnore []string, scanSettings ScanSettings, tokenize TokenizeRules) (uint64, error) {
	state := struct {
		SchemaVersion int
		Profiles      []string
		ScanIgnore    []string
		ScanSettings  ScanSettings
		Tokenize      TokenizeRules
	}{
		SchemaVersion: SchemaVersion,
		Profiles:      append([]string(nil), profiles...),
		ScanIgnore:    append([]string(nil), scanIgnore...),
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
