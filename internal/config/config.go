package config

import (
	"encoding/json"
	"os"
)

// Config holds the root configuration for Repodex.
type Config struct {
	IndexVersion int                `json:"IndexVersion"`
	ProjectType  string             `json:"ProjectType"`
	IncludeExt   []string           `json:"IncludeExt"`
	ExcludeDirs  []string           `json:"ExcludeDirs"`
	Chunk        ChunkingConfig     `json:"Chunk"`
	Token        TokenizationConfig `json:"Token"`
	Limits       LimitsConfig       `json:"Limits"`
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
}

// LimitsConfig controls output limits.
type LimitsConfig struct {
	MaxSnippetBytes int `json:"MaxSnippetBytes"`
}

// DefaultConfig returns a Config populated with defaults.
func DefaultConfig() Config {
	return Config{
		IndexVersion: 1,
		ProjectType:  "ts",
		IncludeExt:   []string{".ts", ".tsx"},
		ExcludeDirs:  []string{"node_modules", "dist", "build", ".next", "coverage", ".git", "out"},
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

// Save writes the config to the provided path in JSON format.
func Save(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Load reads a Config from disk and returns the parsed config along with the raw bytes.
func Load(path string) (Config, []byte, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, nil, err
	}
	return cfg, data, nil
}
