package profile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type goProfile struct{}

func newGoProfile() Profile {
	return goProfile{}
}

func (goProfile) ID() string {
	return "go"
}

func (goProfile) Detect(ctx DetectContext) (bool, error) {
	if _, err := os.Stat(ctx.Join("go.mod")); err == nil {
		return true, nil
	}
	if _, err := os.Stat(ctx.Join("go.work")); err == nil {
		return true, nil
	}
	exts := map[string]struct{}{
		".go": {},
	}
	found := false
	stop := fs.SkipAll
	err := filepath.WalkDir(ctx.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			switch name {
			case ".git", ".repodex", "vendor", "bin", "dist", "build", "out", "tmp":
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if _, ok := exts[ext]; ok {
			found = true
			return stop
		}
		return nil
	})
	if err != nil && !errors.Is(err, stop) {
		return false, err
	}
	return found, nil
}

func (goProfile) Rules() Rules {
	return Rules{
		ScanIgnore: []string{
			"vendor/",
			"bin/",
			"dist/",
			"build/",
			"out/",
			"coverage/",
			"tmp/",
			".cache/",
			".idea/",
			".vscode/",
			".DS_Store",
			"**/*.test",
			"**/*.out",
			"**/*.prof",
			"**/*.trace",
			"**/*.coverprofile",
		},
		IncludeExt: []string{".go", ".mod", ".sum", ".work"},
		Tokenize: TokenizeRules{
			PathStripExts: []string{".go", ".mod", ".sum", ".work"},
			StopWords: []string{
				"break", "case", "chan", "const", "continue", "default", "defer",
				"else", "fallthrough", "for", "func", "go", "goto", "if", "import",
				"interface", "map", "package", "range", "return", "select", "struct",
				"switch", "type", "var", "true", "false", "nil",
			},
		},
	}
}
