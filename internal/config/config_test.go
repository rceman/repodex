package config

import (
	"strings"
	"testing"
)

func TestApplyOverridesRequiresProfiles(t *testing.T) {
	_, _, err := ApplyOverrides(DefaultRuntimeConfig(), UserConfig{})
	if err == nil {
		t.Fatalf("expected profiles error, got nil")
	}
	if !strings.Contains(err.Error(), "profiles") {
		t.Fatalf("expected profiles error, got: %v", err)
	}
}

func TestApplyOverridesAllowsZeroValues(t *testing.T) {
	zero := 0
	userCfg := UserConfig{
		Profiles: []string{"ts_js"},
		Chunk: &UserChunkOverrides{
			OverlapLines: &zero,
		},
	}
	cfg, _, err := ApplyOverrides(DefaultRuntimeConfig(), userCfg)
	if err != nil {
		t.Fatalf("apply overrides: %v", err)
	}
	if cfg.Chunk.OverlapLines != 0 {
		t.Fatalf("expected overlap lines to be 0, got %d", cfg.Chunk.OverlapLines)
	}
}
