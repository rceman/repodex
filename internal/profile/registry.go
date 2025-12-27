package profile

import (
	"fmt"
)

// registry order is stable to keep deterministic rule merging.
var registry = []Profile{
	newNodeProfile(),
	newTSJSProfile(),
}

// DetectResult captures detected profiles and contextual facts.
type DetectResult struct {
	Profiles       []Profile
	HasPackageJSON bool
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
