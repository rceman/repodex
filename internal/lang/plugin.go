package lang

import "github.com/memkit/repodex/internal/config"

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
