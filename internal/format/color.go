package format

import (
	"fmt"
	"strings"
)

// ColorMode describes when ANSI output should be used.
type ColorMode string

const (
	ColorAuto   ColorMode = "auto"
	ColorAlways ColorMode = "always"
	ColorNever  ColorMode = "never"
)

// ColorPolicy captures resolved color behavior.
type ColorPolicy struct {
	Enabled bool
}

// ParseColorMode validates CLI color mode input.
func ParseColorMode(raw string) (ColorMode, error) {
	if raw == "" {
		return ColorAuto, nil
	}
	switch strings.ToLower(raw) {
	case "auto":
		return ColorAuto, nil
	case "always":
		return ColorAlways, nil
	case "never":
		return ColorNever, nil
	default:
		return "", fmt.Errorf("invalid color mode %q (expected auto|always|never)", raw)
	}
}

// ResolveColorPolicy decides if ANSI output should be enabled.
func ResolveColorPolicy(mode ColorMode, noFormat, jsonOut, isTTY, noColor, termDumb bool) ColorPolicy {
	if noColor || noFormat || jsonOut {
		return ColorPolicy{}
	}
	switch mode {
	case ColorNever:
		return ColorPolicy{}
	case ColorAlways:
		return ColorPolicy{Enabled: true}
	case ColorAuto:
		if isTTY && !termDumb {
			return ColorPolicy{Enabled: true}
		}
	}
	return ColorPolicy{}
}
