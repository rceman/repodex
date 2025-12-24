package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/hash"
	"github.com/memkit/repodex/internal/ignore"
	"github.com/memkit/repodex/internal/textutil"
)

// ScannedFile represents a file collected during scanning.
type ScannedFile struct {
	Path    string
	Content []byte
	MTime   int64
	Size    int64
	Hash64  uint64
}

// FileStat contains lightweight file attributes used for status checks.
type FileStat struct {
	Path  string
	MTime int64
	Size  int64
}

// Walk collects all matching files according to configuration and ignore lists.
func Walk(root string, cfg config.Config, ignoreDirs []string) ([]ScannedFile, error) {
	normalizedIgnores := make([]string, 0, len(ignoreDirs)+len(cfg.ExcludeDirs))
	for _, dir := range cfg.ExcludeDirs {
		normalizedIgnores = append(normalizedIgnores, ignore.NormalizePath(dir))
	}
	for _, dir := range ignoreDirs {
		normalizedIgnores = append(normalizedIgnores, ignore.NormalizePath(dir))
	}

	var files []ScannedFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = ignore.NormalizePath(rel)
		if rel == "." {
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() {
			if ignore.IsIgnoredDir(rel, normalizedIgnores) {
				return filepath.SkipDir
			}
			return nil
		}

		if !matchesExt(rel, cfg.IncludeExt) {
			return nil
		}
		if strings.HasSuffix(rel, ".d.ts") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if info.Size() > cfg.Token.MaxFileBytesCode {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content = textutil.NormalizeNewlinesBytes(content)
		hash64 := hash.Sum64(content)

		files = append(files, ScannedFile{
			Path:    rel,
			Content: content,
			MTime:   info.ModTime().Unix(),
			Size:    int64(len(content)),
			Hash64:  hash64,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// WalkMeta collects only path, mtime, and size for matching files.
func WalkMeta(root string, cfg config.Config, ignoreDirs []string) ([]FileStat, error) {
	normalizedIgnores := make([]string, 0, len(ignoreDirs)+len(cfg.ExcludeDirs))
	for _, dir := range cfg.ExcludeDirs {
		normalizedIgnores = append(normalizedIgnores, ignore.NormalizePath(dir))
	}
	for _, dir := range ignoreDirs {
		normalizedIgnores = append(normalizedIgnores, ignore.NormalizePath(dir))
	}

	var files []FileStat
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = ignore.NormalizePath(rel)
		if rel == "." {
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() {
			if ignore.IsIgnoredDir(rel, normalizedIgnores) {
				return filepath.SkipDir
			}
			return nil
		}

		if !matchesExt(rel, cfg.IncludeExt) {
			return nil
		}
		if strings.HasSuffix(rel, ".d.ts") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > cfg.Token.MaxFileBytesCode {
			return nil
		}

		files = append(files, FileStat{
			Path:  rel,
			MTime: info.ModTime().Unix(),
			Size:  info.Size(),
		})

		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func matchesExt(path string, exts []string) bool {
	for _, ext := range exts {
		if strings.HasSuffix(strings.ToLower(path), strings.ToLower(ext)) {
			return true
		}
	}
	return false
}
