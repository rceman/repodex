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
	cfg := config.UserConfig{Profiles: []string{"ts_js", "node"}}
	if err := config.SaveUserConfig(store.ConfigPath(root), cfg); err != nil {
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

func TestFetchEmptyFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(store.Dir(root), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	cfg := config.UserConfig{Profiles: []string{"ts_js", "node"}}
	if err := config.SaveUserConfig(store.ConfigPath(root), cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}

	filePath := filepath.Join(root, "empty.ts")
	if err := os.WriteFile(filePath, []byte(""), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	files := []index.FileEntry{{FileID: 1, Path: "empty.ts"}}
	chunks := []index.ChunkEntry{
		{ChunkID: 1, FileID: 1, Path: "empty.ts", StartLine: 1, EndLine: 5, Snippet: "empty"},
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
	if got.ReturnedFrom != 0 || got.ReturnedTo != 0 {
		t.Fatalf("expected zero range for empty file, got %d-%d", got.ReturnedFrom, got.ReturnedTo)
	}
	if len(got.Lines) != 0 {
		t.Fatalf("expected no lines, got %d", len(got.Lines))
	}
}

func TestFetchTrailingNewline(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(store.Dir(root), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	cfg := config.UserConfig{Profiles: []string{"ts_js", "node"}}
	if err := config.SaveUserConfig(store.ConfigPath(root), cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}

	content := "alpha\nbeta\ngamma\n"
	filePath := filepath.Join(root, "with_newline.ts")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	files := []index.FileEntry{{FileID: 1, Path: "with_newline.ts"}}
	chunks := []index.ChunkEntry{
		{ChunkID: 1, FileID: 1, Path: "with_newline.ts", StartLine: 0, EndLine: 10, Snippet: "with newline"},
	}
	if err := index.Serialize(root, files, chunks, map[string][]uint32{}); err != nil {
		t.Fatalf("serialize failed: %v", err)
	}

	results, err := Fetch(root, []uint32{1}, 120)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(results))
	}
	got := results[0]
	if got.ReturnedFrom != 1 || got.ReturnedTo != 3 {
		t.Fatalf("unexpected returned range %d-%d", got.ReturnedFrom, got.ReturnedTo)
	}
	if len(got.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(got.Lines))
	}
	if got.Lines[0] != "1| alpha" || got.Lines[2] != "3| gamma" {
		t.Fatalf("unexpected line formatting with trailing newline: first=%q last=%q", got.Lines[0], got.Lines[2])
	}
}

func TestFetchCRLF(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(store.Dir(root), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	cfg := config.UserConfig{Profiles: []string{"ts_js", "node"}}
	if err := config.SaveUserConfig(store.ConfigPath(root), cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}

	content := "alpha\r\nbeta\r\ngamma\r\n"
	filePath := filepath.Join(root, "crlf.ts")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	files := []index.FileEntry{{FileID: 1, Path: "crlf.ts"}}
	chunks := []index.ChunkEntry{
		{ChunkID: 1, FileID: 1, Path: "crlf.ts", StartLine: 1, EndLine: 10, Snippet: "crlf file"},
	}
	if err := index.Serialize(root, files, chunks, map[string][]uint32{}); err != nil {
		t.Fatalf("serialize failed: %v", err)
	}

	results, err := Fetch(root, []uint32{1}, 120)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(results))
	}
	got := results[0]
	if got.ReturnedFrom != 1 || got.ReturnedTo != 3 {
		t.Fatalf("unexpected returned range %d-%d", got.ReturnedFrom, got.ReturnedTo)
	}
	if len(got.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(got.Lines))
	}
	if got.Lines[0] != "1| alpha" || got.Lines[2] != "3| gamma" {
		t.Fatalf("unexpected line formatting with CRLF: first=%q last=%q", got.Lines[0], got.Lines[2])
	}
}

func TestResolvePathRejectsDotDotSegment(t *testing.T) {
	root := t.TempDir()
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("eval symlinks failed: %v", err)
	}

	// Even if the cleaned result would be within root, the plan requires rejecting
	// any explicit ".." traversal segment.
	_, err = resolvePath(rootReal, "a/../b.ts")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal detected") {
		t.Fatalf("expected traversal error, got: %v", err)
	}
}

func TestResolvePathRejectsVariousDotDotForms(t *testing.T) {
	root := t.TempDir()
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("eval symlinks failed: %v", err)
	}

	cases := []struct {
		name string
		path string
	}{
		{name: "leading dotdot", path: "../b.ts"},
		{name: "multiple dotdot", path: "a/../../b.ts"},
		{name: "backslash dotdot", path: "a\\..\\b.ts"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolvePath(rootReal, tc.path)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "path traversal detected") {
				t.Fatalf("expected traversal error, got: %v", err)
			}
		})
	}
}
