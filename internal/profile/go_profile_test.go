package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoDetectEarlyStop(t *testing.T) {
	root := t.TempDir()
	match := filepath.Join(root, "pkg", "main.go")
	if err := os.MkdirAll(filepath.Dir(match), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(match, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write match: %v", err)
	}

	poisonDir := filepath.Join(root, "z", "poison")
	if err := os.MkdirAll(poisonDir, 0o000); err != nil {
		t.Fatalf("mkdir poison: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(poisonDir, 0o755)
	})

	profile := goProfile{}
	ctx := DetectContext{Root: root}
	found, err := profile.Detect(ctx)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !found {
		t.Fatalf("expected go profile to be detected")
	}
}
