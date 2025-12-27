package factory

import (
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
