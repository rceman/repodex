package lang

import (
	"path/filepath"
	"strings"

	"github.com/memkit/repodex/internal/config"
)

// ChunkDraft represents a chunk produced during language-specific chunking.
type ChunkDraft struct {
	StartLine uint32
	EndLine   uint32
	Snippet   string
}

// LanguagePlugin defines the interface implemented by language processors.
type LanguagePlugin interface {
	ID() string
	Match(path string) bool
	ChunkFile(path string, content []byte, cfg config.ChunkingConfig, limits config.LimitsConfig) ([]ChunkDraft, error)
	TokenizeChunk(path string, chunkText string, cfg config.TokenizationConfig) []string
}

// SelectPlugin picks the first plugin that matches the provided path.
func SelectPlugin(plugins []LanguagePlugin, extMap map[string]LanguagePlugin, path string) (LanguagePlugin, bool) {
	if extMap != nil {
		ext := strings.ToLower(filepath.Ext(path))
		if ext != "" {
			if p, ok := extMap[ext]; ok {
				return p, true
			}
		}
	}
	for _, p := range plugins {
		if p.Match(path) {
			return p, true
		}
	}
	return nil, false
}
