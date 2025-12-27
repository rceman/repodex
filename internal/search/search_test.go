package search

import (
	"math"
	"os"
	"testing"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/index"
	"github.com/memkit/repodex/internal/store"
)

func TestSearchRanking(t *testing.T) {
	root := t.TempDir()
	files := []index.FileEntry{
		{FileID: 1, Path: "a.ts"},
		{FileID: 2, Path: "b.ts"},
		{FileID: 3, Path: "c.ts"},
	}
	chunks := []index.ChunkEntry{
		{ChunkID: 1, FileID: 1, Path: "a.ts", StartLine: 1, EndLine: 3, Snippet: "alpha only"},
		{ChunkID: 2, FileID: 2, Path: "b.ts", StartLine: 4, EndLine: 8, Snippet: "alpha beta"},
		{ChunkID: 3, FileID: 3, Path: "c.ts", StartLine: 2, EndLine: 6, Snippet: "beta only"},
	}
	postings := map[string][]uint32{
		"alpha": {1, 2},
		"beta":  {2, 3},
	}
	createIndex(t, root, files, chunks, postings)

	results, err := Search(root, "alpha beta", Options{})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].ChunkID != 2 {
		t.Fatalf("expected chunk 2 first, got %d", results[0].ChunkID)
	}
	if results[1].ChunkID != 1 || results[2].ChunkID != 3 {
		t.Fatalf("unexpected order for ties: %d, %d", results[1].ChunkID, results[2].ChunkID)
	}
	idf := math.Log(1 + float64(len(chunks))/2.0)
	want := idf * 2
	if math.Abs(results[0].Score-want) > 1e-9 {
		t.Fatalf("expected score %.6f, got %.6f", want, results[0].Score)
	}
	if len(results[0].Why) != 2 {
		t.Fatalf("expected why terms for chunk 2")
	}
}

func TestSearchMaxPerFile(t *testing.T) {
	root := t.TempDir()
	files := []index.FileEntry{
		{FileID: 1, Path: "same.ts"},
		{FileID: 2, Path: "other.ts"},
	}
	chunks := []index.ChunkEntry{
		{ChunkID: 1, FileID: 1, Path: "same.ts", StartLine: 1, EndLine: 2, Snippet: "alpha"},
		{ChunkID: 2, FileID: 1, Path: "same.ts", StartLine: 3, EndLine: 4, Snippet: "alpha again"},
		{ChunkID: 3, FileID: 1, Path: "same.ts", StartLine: 5, EndLine: 6, Snippet: "alpha more"},
		{ChunkID: 4, FileID: 2, Path: "other.ts", StartLine: 1, EndLine: 2, Snippet: "alpha elsewhere"},
	}
	postings := map[string][]uint32{
		"alpha": {1, 2, 3, 4},
	}
	createIndex(t, root, files, chunks, postings)

	results, err := Search(root, "alpha", Options{})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results after max_per_file filter, got %d", len(results))
	}
	if results[0].ChunkID != 1 || results[1].ChunkID != 2 || results[2].ChunkID != 4 {
		t.Fatalf("unexpected filtered order: %v", results)
	}
}

func createIndex(t *testing.T, root string, files []index.FileEntry, chunks []index.ChunkEntry, postings map[string][]uint32) {
	t.Helper()
	if err := os.MkdirAll(store.Dir(root), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	cfg := config.UserConfig{Profiles: []string{"ts_js", "node"}}
	if err := config.SaveUserConfig(store.ConfigPath(root), cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}
	if err := index.Serialize(root, files, chunks, postings); err != nil {
		t.Fatalf("serialize failed: %v", err)
	}
}
