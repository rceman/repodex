package scan

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/memkit/repodex/internal/hash"
	"github.com/memkit/repodex/internal/profile"
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

// FileRef holds file path and stat metadata without content.
type FileRef struct {
	RelPath string
	AbsPath string
	Size    int64
	MTime   int64
}

// Walk collects all matching files according to effective rules.
func Walk(root string, rules profile.EffectiveRules) ([]ScannedFile, error) {
	candidates, err := collect(root, rules)
	if err != nil {
		return nil, err
	}

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

// WalkRefs enumerates indexable files with stat metadata without reading content.
func WalkRefs(root string, rules profile.EffectiveRules) ([]FileRef, error) {
	candidates, err := collect(root, rules)
	if err != nil {
		return nil, err
	}

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

type candidate struct {
	relPath string
	absPath string
	info    fs.FileInfo
}

func collect(root string, rules profile.EffectiveRules) ([]candidate, error) {
	matcher := newIgnoreMatcher(rules.ScanIgnore)
	var candidates []candidate
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}

		if isHardExcluded(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() {
			if matcher.shouldIgnore(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}

		lowerRel := strings.ToLower(rel)
		if matcher.shouldIgnore(rel, false) {
			return nil
		}
		if profile.IsKnownBinaryExt(lowerRel) {
			return nil
		}
		if !matchesExt(lowerRel, rules.IncludeExt) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > rules.ScanSettings.MaxTextFileSizeBytes {
			return nil
		}
		isBinary, err := profile.IsBinarySniff(path, 4096)
		if err != nil && err != io.EOF {
			return err
		}
		if isBinary {
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
	return candidates, nil
}

func isHardExcluded(rel string) bool {
	if rel == ".repodex" || strings.HasPrefix(rel, ".repodex/") {
		return true
	}
	if rel == ".git" || strings.HasPrefix(rel, ".git/") {
		return true
	}
	return false
}

func matchesExt(lowerPath string, exts []string) bool {
	if len(exts) == 0 {
		return true
	}
	for _, ext := range exts {
		if strings.HasSuffix(lowerPath, strings.ToLower(ext)) {
			return true
		}
	}
	return false
}
