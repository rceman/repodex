package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/profile"
)

func TestConfigHashChangesWithIgnore(t *testing.T) {
	root := t.TempDir()
	userCfg := config.UserConfig{Profiles: []string{"ts_js", "node"}}
	cfg, profiles, err := config.ApplyOverrides(config.DefaultRuntimeConfig(), userCfg)
	if err != nil {
		t.Fatalf("apply overrides: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, ".repodex.ignore"), []byte("tmp/\n"), 0o644); err != nil {
		t.Fatalf("write repodexignore: %v", err)
	}
	rules1, err := profile.BuildEffectiveRules(root, profiles, cfg)
	if err != nil {
		t.Fatalf("rules1: %v", err)
	}
	hash1, err := combinedConfigHash(cfg, rules1.RulesHash)
	if err != nil {
		t.Fatalf("hash1: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, ".repodex.ignore"), []byte("tmp/\n!tmp/keep.ts\n"), 0o644); err != nil {
		t.Fatalf("write repodexignore update: %v", err)
	}
	rules2, err := profile.BuildEffectiveRules(root, profiles, cfg)
	if err != nil {
		t.Fatalf("rules2: %v", err)
	}
	hash2, err := combinedConfigHash(cfg, rules2.RulesHash)
	if err != nil {
		t.Fatalf("hash2: %v", err)
	}
	if hash1 == hash2 {
		t.Fatalf("expected config hash to change after repodexignore update")
	}
}

func TestConfigHashChangesWithOverrides(t *testing.T) {
	cfg := config.DefaultRuntimeConfig()
	rulesHash := uint64(1234)

	hash1, err := combinedConfigHash(cfg, rulesHash)
	if err != nil {
		t.Fatalf("hash1: %v", err)
	}

	cfg.Limits.MaxSnippetBytes++
	hash2, err := combinedConfigHash(cfg, rulesHash)
	if err != nil {
		t.Fatalf("hash2: %v", err)
	}
	if hash1 == hash2 {
		t.Fatalf("expected config hash to change after overrides update")
	}
}
