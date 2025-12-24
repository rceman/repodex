package index

import (
	"strings"
	"testing"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/lang/ts"
	"github.com/memkit/repodex/internal/scan"
	"github.com/memkit/repodex/internal/textutil"
)

func TestBuildNormalizesCRLFLineNumbers(t *testing.T) {
	raw := "function foo() {}\r\nfunction bar() {}\r\nfunction baz() {}"
	cfg := config.Config{
		Chunk: config.ChunkingConfig{MaxLines: 1, OverlapLines: 0, MinChunkLines: 1},
		Token: config.TokenizationConfig{
			MinTokenLen:            1,
			MaxTokenLen:            64,
			DropHexLen:             16,
			TokenizeStringLiterals: true,
		},
		Limits: config.LimitsConfig{MaxSnippetBytes: 200},
	}
	normalized := textutil.NormalizeNewlinesBytes([]byte(raw))
	files := []scan.ScannedFile{{
		Path:    "sample.ts",
		Content: []byte(raw),
		MTime:   1,
		Size:    int64(len(normalized)),
		Hash64:  0,
	}}
	plugin := ts.TSPlugin{}

	fileEntries, chunkEntries, _, err := Build(files, plugin, cfg)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	if len(fileEntries) != 1 {
		t.Fatalf("expected 1 file entry, got %d", len(fileEntries))
	}
	if len(chunkEntries) != 3 {
		t.Fatalf("expected 3 chunk entries, got %d", len(chunkEntries))
	}

	lines := strings.Split(string(normalized), "\n")
	for idx, ch := range chunkEntries {
		expectedStart := uint32(idx + 1)
		expectedEnd := expectedStart
		if ch.StartLine != expectedStart || ch.EndLine != expectedEnd {
			t.Fatalf("unexpected chunk range %d-%d for chunk %d", ch.StartLine, ch.EndLine, idx)
		}
		chunkText := extractText(lines, int(ch.StartLine), int(ch.EndLine))
		if strings.Contains(chunkText, "\r") {
			t.Fatalf("chunk text contains CR characters: %q", chunkText)
		}
	}
}
