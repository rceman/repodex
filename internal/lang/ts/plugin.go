package ts

import (
	"path/filepath"
	"strings"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/lang"
)

// TSPlugin implements LanguagePlugin for TypeScript and TSX.
type TSPlugin struct{}

func (p TSPlugin) ID() string {
	return "ts"
}

func (p TSPlugin) Match(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".ts" && ext != ".tsx" {
		return false
	}
	if strings.HasSuffix(strings.ToLower(path), ".d.ts") {
		return false
	}
	return true
}

func (p TSPlugin) ChunkFile(path string, content []byte, cfg config.ChunkingConfig, limits config.LimitsConfig) ([]lang.ChunkDraft, error) {
	return ChunkFile(path, content, cfg, limits)
}

func (p TSPlugin) TokenizeChunk(path string, chunkText string, cfg config.TokenizationConfig) []string {
	return Tokenize(path, chunkText, cfg)
}
