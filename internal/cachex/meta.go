package cachex

import (
	"errors"
	"os"

	"github.com/memkit/repodex/internal/store"
)

// Meta captures cache-level metadata for validation.
type Meta = store.CacheMeta

// LoadMeta reads the cache metadata if present.
func LoadMeta(root string) (Meta, bool, error) {
	meta, err := store.LoadMeta(store.MetaPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return Meta{}, false, nil
	}
	if err != nil {
		return Meta{}, false, err
	}
	if meta.Cache == nil {
		return Meta{}, false, nil
	}
	return *meta.Cache, true, nil
}

// EnsureMeta verifies cache metadata, purging the current cache on mismatch.
func EnsureMeta(root string, want Meta) (bool, error) {
	want.CacheVersion = CacheVersion
	want.SchemaVersion = store.SchemaVersion

	existing, ok, err := LoadMeta(root)
	if err != nil {
		return false, err
	}
	if ok && !metaEqual(existing, want) {
		if err := Purge(root); err != nil {
			return false, err
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
