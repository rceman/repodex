package cachex

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/memkit/repodex/internal/store"
)

const CacheVersion = "v1"

// CacheEntry represents a serialized per-file cache record.
type CacheEntry struct {
	RelPath string       `json:"rel_path"`
	Size    int64        `json:"size"`
	MTime   int64        `json:"mtime"`
	Hash64  uint64       `json:"hash64"`
	Chunks  []LocalChunk `json:"chunks"`
	Tokens  [][]string   `json:"tokens"`
}

// LocalChunk mirrors a chunk without a global ChunkID.
type LocalChunk struct {
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Snippet string `json:"snippet"`
}

// CacheDir returns the v1 cache directory under the repo root.
func CacheDir(root string) string {
	return filepath.Join(store.Dir(root), "cache", CacheVersion)
}

// PurgeV1 removes the v1 cache directory entirely.
func PurgeV1(root string) error {
	return os.RemoveAll(CacheDir(root))
}

// Save writes a cache entry for the provided file.
func Save(root string, entry CacheEntry) error {
	dir := CacheDir(root)
	entry.RelPath = filepath.ToSlash(entry.RelPath)
	path := cachePath(dir, entry.RelPath)
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return writeFileAtomicReplace(path, data, 0o644)
}

// LoadByPath reads a cache entry for the given relative path without stat validation.
func LoadByPath(root string, relPath string) (CacheEntry, bool, error) {
	normalized := filepath.ToSlash(relPath)
	path := cachePath(CacheDir(root), normalized)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return CacheEntry{}, false, nil
	}
	if err != nil {
		return CacheEntry{}, false, err
	}
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return CacheEntry{}, false, err
	}
	if filepath.ToSlash(entry.RelPath) != normalized {
		return CacheEntry{}, false, nil
	}
	if len(entry.Chunks) != len(entry.Tokens) {
		return CacheEntry{}, false, nil
	}
	return entry, true, nil
}

func cachePath(dir string, relPath string) string {
	sum := sha1.Sum([]byte(filepath.ToSlash(relPath)))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".json")
}

func writeFileAtomicReplace(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		if _, statErr := os.Stat(path); statErr == nil {
			if removeErr := os.Remove(path); removeErr != nil {
				_ = os.Remove(tmp)
				return removeErr
			}
			if retryErr := os.Rename(tmp, path); retryErr == nil {
				return nil
			} else {
				_ = os.Remove(tmp)
				return retryErr
			}
		}
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
