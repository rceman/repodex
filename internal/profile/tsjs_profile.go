package profile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type tsjsProfile struct{}

func newTSJSProfile() Profile {
	return tsjsProfile{}
}

func (tsjsProfile) ID() string {
	return "ts_js"
}

func (tsjsProfile) Detect(ctx DetectContext) (bool, error) {
	if _, err := os.Stat(ctx.Join("tsconfig.json")); err == nil {
		return true, nil
	}
	exts := map[string]struct{}{
		".ts":  {},
		".tsx": {},
		".js":  {},
		".jsx": {},
		".mjs": {},
		".cjs": {},
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
			case ".git", ".repodex", "node_modules", "dist", "build":
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

func (tsjsProfile) Rules() Rules {
	return Rules{
		ScanIgnore: []string{"**/*.map"},
		Tokenize: TokenizeRules{
			PathStripSuffixes: []string{".d.ts.map", ".d.tsx", ".d.ts"},
			PathStripExts:     []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"},
		},
	}
}
