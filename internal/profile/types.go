package profile

import (
	"path/filepath"

	"github.com/memkit/repodex/internal/config"
)

// SchemaVersion tracks rules schema to force rebuilds on incompatible changes.
const SchemaVersion = 1

// Profile defines detection and rule hooks for a technology profile.
type Profile interface {
	ID() string
	Detect(ctx DetectContext) (bool, error)
	Rules() Rules
}

// Rules captures scan and tokenization rules for a profile.
type Rules struct {
	ScanIgnore []string
	IncludeExt []string
	Tokenize   TokenizeRules
}

// TokenizeRules describes path/token tweaks.
type TokenizeRules struct {
	PathStripSuffixes []string
	PathStripExts     []string
	StopWords         []string
	MinTokenLen       int
	MaxTokenLen       int
	DropHexLen        int
	AllowShortTokens  []string
	TokenizeStrings   *bool
}

// DetectContext provides helpers for profile detection.
type DetectContext struct {
	Root string
}

// Join returns an absolute path inside the repo root.
func (c DetectContext) Join(parts ...string) string {
	all := []string{c.Root}
	all = append(all, parts...)
	return filepath.Join(all...)
}

// EffectiveRules represents the merged scan and tokenization rules.
type EffectiveRules struct {
	ScanIgnore   []string
	IncludeExt   []string
	Tokenize     TokenizeRules
	TokenConfig  config.TokenizationConfig
	Profiles     []string
	ScanSettings ScanSettings
	RulesHash    uint64
}

// ScanSettings captures scan-level knobs.
type ScanSettings struct {
	MaxTextFileSizeBytes int64
}
