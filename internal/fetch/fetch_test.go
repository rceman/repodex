package fetch

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/index"
	"github.com/memkit/repodex/internal/store"
)

func TestFetchRespectsMaxLines(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(store.Dir(root), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	cfg := config.DefaultConfig()
	if err := config.Save(store.ConfigPath(root), cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}

	var lines []string
	for i := 1; i <= 150; i++ {
		lines = append(lines, "line "+strconv.Itoa(i))
	}
	filePath := filepath.Join(root, "file.ts")
	if err := os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	files := []index.FileEntry{{FileID: 1, Path: "file.ts"}}
	chunks := []index.ChunkEntry{
		{ChunkID: 1, FileID: 1, Path: "file.ts", StartLine: 1, EndLine: 150, Snippet: "long chunk"},
	}
	if err := index.Serialize(root, files, chunks, map[string][]uint32{}); err != nil {
		t.Fatalf("serialize failed: %v", err)
	}

	results, err := Fetch(root, []uint32{1}, 10)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(results))
	}
	got := results[0]
	if got.ReturnedFrom != 1 || got.ReturnedTo != 10 {
		t.Fatalf("unexpected returned range %d-%d", got.ReturnedFrom, got.ReturnedTo)
	}
	if len(got.Lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(got.Lines))
	}
	if got.Lines[0] != "1| line 1" || got.Lines[9] != "10| line 10" {
		t.Fatalf("unexpected line formatting: first=%q last=%q", got.Lines[0], got.Lines[9])
	}
}
