package cachex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/memkit/repodex/internal/store"
)

// Meta captures cache-level metadata for validation.
type Meta struct {
	CacheVersion  string   `json:"cache_version"`
	SchemaVersion int      `json:"schema_version"`
	ConfigHash    uint64   `json:"config_hash"`
	Profiles      []string `json:"profiles"`
}

// MetaPath returns the path to the cache metadata file.
func MetaPath(root string) string {
	return filepath.Join(CacheDir(root), "meta.json")
}

// LoadMeta reads the cache metadata if present.
func LoadMeta(root string) (Meta, bool, error) {
	path := MetaPath(root)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Meta{}, false, nil
	}
	if err != nil {
		return Meta{}, false, err
	}
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return Meta{}, false, err
	}
	return meta, true, nil
}

// SaveMeta writes cache metadata atomically.
func SaveMeta(root string, meta Meta) error {
	path := MetaPath(root)
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return writeFileAtomicReplace(path, data, 0o644)
}

// EnsureMeta verifies cache metadata, purging the current cache on mismatch.
func EnsureMeta(root string, want Meta) (bool, error) {
	want.CacheVersion = CacheVersion
	want.SchemaVersion = store.SchemaVersion

	existing, ok, err := LoadMeta(root)
	if err != nil {
		return false, err
	}
	if !ok {
		if err := SaveMeta(root, want); err != nil {
			return false, err
		}
		return false, nil
	}
	if !metaEqual(existing, want) {
		if err := Purge(root); err != nil {
			return false, err
		}
		if err := SaveMeta(root, want); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

func metaEqual(a, b Meta) bool {
	if a.CacheVersion != b.CacheVersion ||
		a.SchemaVersion != b.SchemaVersion ||
		a.ConfigHash != b.ConfigHash {
		return false
	}
	if len(a.Profiles) != len(b.Profiles) {
		return false
	}
	for i := range a.Profiles {
		if a.Profiles[i] != b.Profiles[i] {
			return false
		}
	}
	return true
}
