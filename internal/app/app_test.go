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
