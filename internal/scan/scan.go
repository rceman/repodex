package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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

// FileRef holds file path and stat metadata without content.
type FileRef struct {
	RelPath string
	AbsPath string
	Size    int64
	MTime   int64
}

// Walk collects all matching files according to configuration and ignore lists.
func Walk(root string, cfg config.Config, ignoreDirs []string) ([]ScannedFile, error) {
	normalizedIgnores := normalizeIgnores(cfg, ignoreDirs)

	type candidate struct {
		relPath string
		absPath string
		info    fs.FileInfo
	}

	var candidates []candidate
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

		candidates = append(candidates, candidate{
			relPath: rel,
			absPath: path,
			info:    info,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].relPath < candidates[j].relPath
	})

	files := make([]ScannedFile, 0, len(candidates))
	for _, cand := range candidates {
		content, err := os.ReadFile(cand.absPath)
		if err != nil {
			return nil, err
		}
		content = textutil.NormalizeNewlinesBytes(content)
		hash64 := hash.Sum64(content)

		files = append(files, ScannedFile{
			Path:    cand.relPath,
			Content: content,
			MTime:   cand.info.ModTime().Unix(),
			Size:    cand.info.Size(),
			Hash64:  hash64,
		})
	}
	return files, nil
}

// WalkMeta collects only path, mtime, and size for matching files.
func WalkMeta(root string, cfg config.Config, ignoreDirs []string) ([]FileStat, error) {
	normalizedIgnores := normalizeIgnores(cfg, ignoreDirs)

	type candidate struct {
		relPath string
		info    fs.FileInfo
	}

	var candidates []candidate
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

		candidates = append(candidates, candidate{
			relPath: rel,
			info:    info,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].relPath < candidates[j].relPath
	})

	files := make([]FileStat, 0, len(candidates))
	for _, cand := range candidates {
		files = append(files, FileStat{
			Path:  cand.relPath,
			MTime: cand.info.ModTime().Unix(),
			Size:  cand.info.Size(),
		})
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

// WalkRefs enumerates indexable files with stat metadata without reading content.
func WalkRefs(root string, cfg config.Config, ignoreDirs []string) ([]FileRef, error) {
	normalizedIgnores := normalizeIgnores(cfg, ignoreDirs)

	type candidate struct {
		relPath string
		absPath string
		info    fs.FileInfo
	}

	var candidates []candidate
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

		candidates = append(candidates, candidate{
			relPath: rel,
			absPath: path,
			info:    info,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].relPath < candidates[j].relPath
	})

	refs := make([]FileRef, 0, len(candidates))
	for _, cand := range candidates {
		refs = append(refs, FileRef{
			RelPath: cand.relPath,
			AbsPath: cand.absPath,
			Size:    cand.info.Size(),
			MTime:   cand.info.ModTime().Unix(),
		})
	}
	return refs, nil
}

func normalizeIgnores(cfg config.Config, ignoreDirs []string) []string {
	normalized := make([]string, 0, len(ignoreDirs)+len(cfg.ExcludeDirs))
	for _, dir := range cfg.ExcludeDirs {
		normalized = append(normalized, ignore.NormalizePath(dir))
	}
	for _, dir := range ignoreDirs {
		normalized = append(normalized, ignore.NormalizePath(dir))
	}
	return normalized
}
