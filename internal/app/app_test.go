package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunInitForceOverwritesRepodexDir(t *testing.T) {
	root := t.TempDir()

	if err := runInit(root, false); err != nil {
		t.Fatalf("initial init failed: %v", err)
	}

	sentinel := filepath.Join(root, ".repodex", "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("sentinel"), 0o644); err != nil {
		t.Fatalf("failed to write sentinel: %v", err)
	}

	if err := runInit(root, true); err != nil {
		t.Fatalf("force init failed: %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil || !os.IsNotExist(err) {
		t.Fatalf("sentinel should be removed after force init")
	}

	required := []string{
		".repodex/config.json",
		".repodex/ignore",
		".repodex/meta.json",
	}
	for _, rel := range required {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected %s to exist: %v", rel, err)
		}
	}
}

func TestComputeStatusMissingIndexArtifact(t *testing.T) {
	root := t.TempDir()
	repodexDir := filepath.Join(root, ".repodex")
	if err := os.Mkdir(repodexDir, 0o755); err != nil {
		t.Fatalf("failed to create repodex dir: %v", err)
	}

	// Create only the files that should exist; omit chunks.dat to simulate a partial index.
	touch := []string{
		filepath.Join(repodexDir, "meta.json"),
		filepath.Join(repodexDir, "files.dat"),
		filepath.Join(repodexDir, "terms.dat"),
		filepath.Join(repodexDir, "postings.dat"),
	}
	for _, path := range touch {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("failed to create %s: %v", path, err)
		}
	}

	resp, err := computeStatus(root)
	if err != nil {
		t.Fatalf("computeStatus returned error: %v", err)
	}

	if resp.Indexed {
		t.Fatalf("expected Indexed to be false when an index artifact is missing")
	}
	if !resp.Dirty {
		t.Fatalf("expected Dirty to be true when an index artifact is missing")
	}
	if resp.ChangedFiles != 0 {
		t.Fatalf("expected ChangedFiles to be 0 for missing artifact, got %d", resp.ChangedFiles)
	}
}
