package profile

import (
	"fmt"
)

// registry order is stable to keep deterministic rule merging.
var registry = []Profile{
	newNodeProfile(),
	newTSJSProfile(),
	newGoProfile(),
}

// DetectResult captures detected profiles and contextual facts.
type DetectResult struct {
	Profiles       []Profile
	HasPackageJSON bool
}

// ResolveProfiles returns profiles in the provided order, validating IDs.
func ResolveProfiles(ids []string) ([]Profile, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("profiles list is empty")
	}
	byID := make(map[string]Profile, len(registry))
	for _, p := range registry {
		byID[p.ID()] = p
	}
	out := make([]Profile, 0, len(ids))
	for _, id := range ids {
		p, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown profile %q", id)
		}
		out = append(out, p)
	}
	return out, nil
}

// DetectProfiles runs detection in registry order.
func DetectProfiles(ctx DetectContext) (DetectResult, error) {
	var enabled []Profile
	hasPackageJSON := false
	for _, p := range registry {
		match, err := p.Detect(ctx)
		if err != nil {
			return DetectResult{}, fmt.Errorf("%s detect failed: %w", p.ID(), err)
		}
		if match {
			if p.ID() == "node" {
				hasPackageJSON = true
			}
			enabled = append(enabled, p)
		}
	}
	return DetectResult{Profiles: enabled, HasPackageJSON: hasPackageJSON}, nil
}
