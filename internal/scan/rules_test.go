package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/profile"
)

func newTestConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.IncludeExt = nil
	cfg.Scan.MaxTextFileSizeBytes = 1024
	return cfg
}

func TestKnownBinaryExtFastPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatalf("write png: %v", err)
	}
	cfg := newTestConfig()
	rules, err := profile.BuildEffectiveRules(root, cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	results, err := Walk(root, cfg, rules)
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
	rules, err := profile.BuildEffectiveRules(root, cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	results, err := Walk(root, cfg, rules)
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
	rules, err := profile.BuildEffectiveRules(root, cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	results, err := Walk(root, cfg, rules)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected size gate skip, got %d files", len(results))
	}
}

func TestSVGDefaultIgnoreWithOverride(t *testing.T) {
	root := t.TempDir()
	svgPath := filepath.Join(root, "diagram.svg")
	if err := os.WriteFile(svgPath, []byte("<svg></svg>"), 0o644); err != nil {
		t.Fatalf("write svg: %v", err)
	}
	cfg := newTestConfig()
	cfg.IncludeExt = []string{".svg"}

	rules, err := profile.BuildEffectiveRules(root, cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	results, err := Walk(root, cfg, rules)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected svg to be ignored by default")
	}

	if err := os.WriteFile(filepath.Join(root, ".scanignore"), []byte("!**/*.svg\n"), 0o644); err != nil {
		t.Fatalf("write scanignore: %v", err)
	}
	rulesOverride, err := profile.BuildEffectiveRules(root, cfg)
	if err != nil {
		t.Fatalf("rules override: %v", err)
	}
	results, err = Walk(root, cfg, rulesOverride)
	if err != nil {
		t.Fatalf("walk override: %v", err)
	}
	if len(results) != 1 || results[0].Path != "diagram.svg" {
		t.Fatalf("expected svg to be included after override, got %+v", results)
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
	if err := os.WriteFile(filepath.Join(root, ".scanignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatalf("write scanignore: %v", err)
	}
	cfg := newTestConfig()
	cfg.IncludeExt = []string{".ts"}
	rules, err := profile.BuildEffectiveRules(root, cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	results, err := Walk(root, cfg, rules)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected nested node_modules to be ignored, got %+v", results)
	}
}

func TestPackageLockIgnoreWithOverride(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "package-lock.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write package-lock.json: %v", err)
	}
	cfg := newTestConfig()
	cfg.IncludeExt = []string{".json"}

	rules, err := profile.BuildEffectiveRules(root, cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	results, err := Walk(root, cfg, rules)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(results) != 1 || results[0].Path != "package.json" {
		t.Fatalf("expected package-lock.json ignored, got %+v", results)
	}

	if err := os.WriteFile(filepath.Join(root, ".scanignore"), []byte("!package-lock.json\n"), 0o644); err != nil {
		t.Fatalf("write scanignore: %v", err)
	}
	rulesOverride, err := profile.BuildEffectiveRules(root, cfg)
	if err != nil {
		t.Fatalf("rules override: %v", err)
	}
	results, err = Walk(root, cfg, rulesOverride)
	if err != nil {
		t.Fatalf("walk override: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected package-lock.json restored, got %+v", results)
	}
}

func TestRulesHashInvalidation(t *testing.T) {
	root := t.TempDir()
	cfg := newTestConfig()

	rules1, err := profile.BuildEffectiveRules(root, cfg)
	if err != nil {
		t.Fatalf("rules1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scanignore"), []byte("tmp/\n"), 0o644); err != nil {
		t.Fatalf("write scanignore: %v", err)
	}
	rules2, err := profile.BuildEffectiveRules(root, cfg)
	if err != nil {
		t.Fatalf("rules2: %v", err)
	}
	if rules1.RulesHash == rules2.RulesHash {
		t.Fatalf("expected rules hash to change after scanignore update")
	}

	tokenDir := filepath.Join(root, ".repodex")
	if err := os.MkdirAll(tokenDir, 0o755); err != nil {
		t.Fatalf("mkdir repodex: %v", err)
	}
	tokenOverride := `{
  "path_strip_exts": {
    "mode": "replace",
    "values": [".foo"]
  }
}`
	if err := os.WriteFile(filepath.Join(tokenDir, "tokenize.json"), []byte(tokenOverride), 0o644); err != nil {
		t.Fatalf("write tokenize override: %v", err)
	}
	rules3, err := profile.BuildEffectiveRules(root, cfg)
	if err != nil {
		t.Fatalf("rules3: %v", err)
	}
	if rules2.RulesHash == rules3.RulesHash {
		t.Fatalf("expected rules hash to change after tokenize override update")
	}
}
