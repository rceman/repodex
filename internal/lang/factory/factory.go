package factory

import (
	"strings"

	"github.com/memkit/repodex/internal/lang"
	golang "github.com/memkit/repodex/internal/lang/go"
	"github.com/memkit/repodex/internal/lang/ts"
)

// PluginsForProfiles returns language plugins in the order of the provided profiles.
func PluginsForProfiles(profiles []string) []lang.LanguagePlugin {
	seen := make(map[string]struct{})
	var out []lang.LanguagePlugin
	for _, p := range profiles {
		switch p {
		case "ts_js":
			plugin := ts.TSPlugin{}
			if _, ok := seen[plugin.ID()]; ok {
				continue
			}
			seen[plugin.ID()] = struct{}{}
			out = append(out, plugin)
		case "go":
			plugin := golang.GoPlugin{}
			if _, ok := seen[plugin.ID()]; ok {
				continue
			}
			seen[plugin.ID()] = struct{}{}
			out = append(out, plugin)
		}
	}
	return out
}

// ExtensionMapForProfiles returns a lowercased extension map keyed by ".ext".
func ExtensionMapForProfiles(profiles []string) map[string]lang.LanguagePlugin {
	plugins := PluginsForProfiles(profiles)
	out := make(map[string]lang.LanguagePlugin, len(plugins))
	for _, plugin := range plugins {
		switch plugin.ID() {
		case "ts":
			addExts(out, plugin, ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs")
		case "go":
			addExts(out, plugin, ".go", ".mod", ".work")
		}
	}
	return out
}

func addExts(dst map[string]lang.LanguagePlugin, plugin lang.LanguagePlugin, exts ...string) {
	for _, ext := range exts {
		normalized := strings.ToLower(strings.TrimSpace(ext))
		if normalized == "" {
			continue
		}
		if !strings.HasPrefix(normalized, ".") {
			normalized = "." + normalized
		}
		dst[normalized] = plugin
	}
}
