package factory

import (
	"fmt"

	"github.com/memkit/repodex/internal/lang"
	"github.com/memkit/repodex/internal/lang/ts"
)

// FromProjectType returns a language plugin based on project type.
func FromProjectType(projectType string) (lang.LanguagePlugin, error) {
	switch projectType {
	case "ts":
		return ts.TSPlugin{}, nil
	default:
		return nil, fmt.Errorf("unsupported project type: %s", projectType)
	}
}
