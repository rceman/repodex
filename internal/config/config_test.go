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
