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
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return true
	default:
		return false
	}
}

func (p TSPlugin) ChunkFile(path string, content []byte, cfg config.ChunkingConfig, limits config.LimitsConfig) ([]lang.ChunkDraft, error) {
	return ChunkFile(path, content, cfg, limits)
}

func (p TSPlugin) TokenizeChunk(path string, chunkText string, cfg config.TokenizationConfig) []string {
	return Tokenize(path, chunkText, cfg)
}
