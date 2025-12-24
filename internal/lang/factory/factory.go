package factory

import (
	"fmt"

	"github.com/memkit/repodex/internal/lang"
	"github.com/memkit/repodex/internal/lang/ts"
)

const ProjectTypeTS = "ts"

// FromProjectType returns a language plugin based on project type.
func FromProjectType(projectType string) (lang.LanguagePlugin, error) {
	switch projectType {
	case ProjectTypeTS:
		return ts.TSPlugin{}, nil
	default:
		return nil, fmt.Errorf("unsupported project type: %s", projectType)
	}
}
