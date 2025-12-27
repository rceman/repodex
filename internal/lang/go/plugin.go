package golang

import (
	"path/filepath"
	"strings"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/lang"
)

// GoPlugin implements LanguagePlugin for Go.
type GoPlugin struct{}

func (p GoPlugin) ID() string {
	return "go"
}

func (p GoPlugin) Match(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".mod", ".work":
		return true
	default:
		return false
	}
}

func (p GoPlugin) ChunkFile(path string, content []byte, cfg config.ChunkingConfig, limits config.LimitsConfig) ([]lang.ChunkDraft, error) {
	return ChunkFile(path, content, cfg, limits)
}

func (p GoPlugin) TokenizeChunk(path string, chunkText string, cfg config.TokenizationConfig) []string {
	return Tokenize(path, chunkText, cfg)
}
