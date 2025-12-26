package ts

import (
	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/tokenize"
)

// Tokenize extracts tokens from chunk text and file path using the shared tokenizer.
func Tokenize(path string, chunkText string, cfg config.TokenizationConfig) []string {
	return tokenize.New(cfg).WithPath(path, chunkText)
}
