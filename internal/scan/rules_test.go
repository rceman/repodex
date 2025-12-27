package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/profile"
)

func newTestConfig() config.Config {
	cfg := config.DefaultRuntimeConfig()
	cfg.Scan.MaxTextFileSizeBytes = 1024
	return cfg
}

func newTestProfiles() []string {
	return []string{"ts_js", "node"}
}

func TestKnownBinaryExtFastPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatalf("write png: %v", err)
	}
	cfg := newTestConfig()
	rules, err := profile.BuildEffectiveRules(root, newTestProfiles(), cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	results, err := Walk(root, rules)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected png to be skipped, got %d files", len(results))
	}
}

func TestBinarySniffFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "blob.binlike"), []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("write blob: %v", err)
	}
	cfg := newTestConfig()
	rules, err := profile.BuildEffectiveRules(root, newTestProfiles(), cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	results, err := Walk(root, rules)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected binary sniff skip, got %d files", len(results))
	}
}

func TestMaxTextFileSizeGate(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "large.txt"), []byte("0123456789"), 0o644); err != nil {
		t.Fatalf("write large: %v", err)
	}
	cfg := newTestConfig()
	cfg.Scan.MaxTextFileSizeBytes = 4
	rules, err := profile.BuildEffectiveRules(root, newTestProfiles(), cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	results, err := Walk(root, rules)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected size gate skip, got %d files", len(results))
	}
}

func TestIgnoreNegationRestoresInclusion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "diagram.ts")
	if err := os.WriteFile(path, []byte("const diagram = 1"), 0o644); err != nil {
		t.Fatalf("write diagram: %v", err)
	}
	cfg := newTestConfig()

	if err := os.WriteFile(filepath.Join(root, ".repodex.ignore"), []byte("**/*.ts\n!diagram.ts\n"), 0o644); err != nil {
		t.Fatalf("write repodexignore: %v", err)
	}
	rulesOverride, err := profile.BuildEffectiveRules(root, newTestProfiles(), cfg)
	if err != nil {
		t.Fatalf("rules override: %v", err)
	}
	results, err := Walk(root, rulesOverride)
	if err != nil {
		t.Fatalf("walk override: %v", err)
	}
	if len(results) != 1 || results[0].Path != "diagram.ts" {
		t.Fatalf("expected diagram.ts to be included after override, got %+v", results)
	}
}

func TestNestedDirIgnoreMatchesAnywhere(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "apps", "foo", "node_modules", "a.ts")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("const a = 1"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".repodex.ignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatalf("write repodexignore: %v", err)
	}
	cfg := newTestConfig()
	rules, err := profile.BuildEffectiveRules(root, newTestProfiles(), cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	results, err := Walk(root, rules)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected nested node_modules to be ignored, got %+v", results)
	}
}

func TestIgnoreOverrideOrder(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "keep.ts"), []byte("const keep = true"), 0o644); err != nil {
		t.Fatalf("write keep: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "drop.ts"), []byte("const drop = true"), 0o644); err != nil {
		t.Fatalf("write drop: %v", err)
	}
	cfg := newTestConfig()

	if err := os.WriteFile(filepath.Join(root, ".repodex.ignore"), []byte("**/*.ts\n!keep.ts\n"), 0o644); err != nil {
		t.Fatalf("write repodexignore: %v", err)
	}
	rules, err := profile.BuildEffectiveRules(root, newTestProfiles(), cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	results, err := Walk(root, rules)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(results) != 1 || results[0].Path != "keep.ts" {
		t.Fatalf("expected only keep.ts included, got %+v", results)
	}
}

func TestRulesHashInvalidation(t *testing.T) {
	root := t.TempDir()
	cfg := newTestConfig()

	rules1, err := profile.BuildEffectiveRules(root, newTestProfiles(), cfg)
	if err != nil {
		t.Fatalf("rules1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".repodex.ignore"), []byte("tmp/\n"), 0o644); err != nil {
		t.Fatalf("write repodexignore: %v", err)
	}
	rules2, err := profile.BuildEffectiveRules(root, newTestProfiles(), cfg)
	if err != nil {
		t.Fatalf("rules2: %v", err)
	}
	if rules1.RulesHash == rules2.RulesHash {
		t.Fatalf("expected rules hash to change after repodexignore update")
	}
}

func TestHardExcludeRepodexAndGit(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, ".repodex", "keep.ts"),
		filepath.Join(root, ".git", "skip.ts"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("const x = 1"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".repodex.ignore"), []byte("\n"), 0o644); err != nil {
		t.Fatalf("write repodexignore: %v", err)
	}

	cfg := newTestConfig()
	rules, err := profile.BuildEffectiveRules(root, newTestProfiles(), cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	results, err := Walk(root, rules)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected hard-excluded paths to be skipped, got %+v", results)
	}
}
