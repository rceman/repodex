package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config holds the root configuration for Repodex.
type Config struct {
	Scan   ScanConfig
	Chunk  ChunkingConfig
	Token  TokenizationConfig
	Limits LimitsConfig
}

// ChunkingConfig configures how files are chunked.
type ChunkingConfig struct {
	MaxLines      int `json:"MaxLines"`
	OverlapLines  int `json:"OverlapLines"`
	MinChunkLines int `json:"MinChunkLines"`
}

// TokenizationConfig configures token extraction.
type TokenizationConfig struct {
	MinTokenLen            int      `json:"MinTokenLen"`
	MaxTokenLen            int      `json:"MaxTokenLen"`
	DropHexLen             int      `json:"DropHexLen"`
	AllowShortTokens       []string `json:"AllowShortTokens"`
	StopWords              []string `json:"StopWords"`
	TokenizeStringLiterals bool     `json:"TokenizeStringLiterals"`
	MaxFileBytesCode       int64    `json:"MaxFileBytesCode"`
	PathStripSuffixes      []string `json:"PathStripSuffixes"`
	PathStripExts          []string `json:"PathStripExts"`
}

// ScanConfig controls scanning behavior.
type ScanConfig struct {
	MaxTextFileSizeBytes int64 `json:"MaxTextFileSizeBytes"`
}

// LimitsConfig controls output limits.
type LimitsConfig struct {
	MaxSnippetBytes int `json:"MaxSnippetBytes"`
}

const IndexVersion = 1

// UserConfig holds user overrides stored on disk.
type UserConfig struct {
	Profiles []string             `json:"profiles"`
	Scan     *UserScanOverrides   `json:"scan,omitempty"`
	Chunk    *UserChunkOverrides  `json:"chunk,omitempty"`
	Limits   *UserLimitsOverrides `json:"limits,omitempty"`
}

// UserChunkOverrides describes chunking overrides for user config.
type UserChunkOverrides struct {
	MaxLines      *int `json:"maxLines,omitempty"`
	OverlapLines  *int `json:"overlapLines,omitempty"`
	MinChunkLines *int `json:"minChunkLines,omitempty"`
}

// UserScanOverrides describes scan overrides for user config.
type UserScanOverrides struct {
	MaxTextFileSizeBytes *int64 `json:"maxTextFileSizeBytes,omitempty"`
}

// UserLimitsOverrides describes output limit overrides for user config.
type UserLimitsOverrides struct {
	MaxSnippetBytes *int `json:"maxSnippetBytes,omitempty"`
}

// DefaultRuntimeConfig returns a Config populated with defaults.
func DefaultRuntimeConfig() Config {
	return Config{
		Scan: ScanConfig{
			MaxTextFileSizeBytes: 1024 * 1024,
		},
		Chunk: ChunkingConfig{
			MaxLines:      200,
			OverlapLines:  20,
			MinChunkLines: 20,
		},
		Token: TokenizationConfig{
			MinTokenLen:            3,
			MaxTokenLen:            64,
			DropHexLen:             16,
			AllowShortTokens:       []string{"api", "jwt", "url", "ui", "css", "tsx", "jsx", "dom", "id"},
			StopWords:              defaultStopWords(),
			TokenizeStringLiterals: true,
			MaxFileBytesCode:       2 * 1024 * 1024,
		},
		Limits: LimitsConfig{
			MaxSnippetBytes: 800,
		},
	}
}

func defaultStopWords() []string {
	return []string{
		"const", "let", "var", "function", "return", "export", "import", "from",
		"class", "interface", "type", "enum", "extends", "implements", "new",
		"this", "super", "public", "private", "protected", "readonly", "async",
		"await", "if", "else", "switch", "case", "for", "while", "do", "break",
		"continue", "try", "catch", "finally", "throw", "true", "false", "null",
		"undefined",
	}
}

// SaveUserConfig writes the user config to disk.
func SaveUserConfig(path string, cfg UserConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadUserConfig reads a user config from disk and returns the parsed config along with raw bytes.
func LoadUserConfig(path string) (UserConfig, []byte, error) {
	var cfg UserConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, nil, err
	}
	return cfg, data, nil
}

// ApplyOverrides merges a user config into runtime defaults.
func ApplyOverrides(defaults Config, user UserConfig) (Config, []string, error) {
	profiles := sanitizeProfiles(user.Profiles)
	if len(profiles) == 0 {
		return Config{}, nil, fmt.Errorf("profiles are required in .repodex.json")
	}
	cfg := defaults
	if user.Scan != nil && user.Scan.MaxTextFileSizeBytes != nil {
		cfg.Scan.MaxTextFileSizeBytes = *user.Scan.MaxTextFileSizeBytes
	}
	if user.Chunk != nil {
		if user.Chunk.MaxLines != nil {
			cfg.Chunk.MaxLines = *user.Chunk.MaxLines
		}
		if user.Chunk.OverlapLines != nil {
			cfg.Chunk.OverlapLines = *user.Chunk.OverlapLines
		}
		if user.Chunk.MinChunkLines != nil {
			cfg.Chunk.MinChunkLines = *user.Chunk.MinChunkLines
		}
	}
	if user.Limits != nil && user.Limits.MaxSnippetBytes != nil {
		cfg.Limits.MaxSnippetBytes = *user.Limits.MaxSnippetBytes
	}
	return cfg, profiles, nil
}

func sanitizeProfiles(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
