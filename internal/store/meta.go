package store

import (
	"encoding/json"
	"os"
	"time"
)

// Meta captures persisted index metadata.
type Meta struct {
	IndexVersion  int    `json:"IndexVersion"`
	IndexedAtUnix int64  `json:"IndexedAtUnix"`
	FileCount     int    `json:"FileCount"`
	ChunkCount    int    `json:"ChunkCount"`
	TermCount     int    `json:"TermCount"`
	ConfigHash    uint64 `json:"ConfigHash"`
}

// NewMeta builds a Meta with the supplied counts and current timestamp.
func NewMeta(indexVersion int, fileCount, chunkCount, termCount int, configHash uint64) Meta {
	return Meta{
		IndexVersion:  indexVersion,
		IndexedAtUnix: time.Now().Unix(),
		FileCount:     fileCount,
		ChunkCount:    chunkCount,
		TermCount:     termCount,
		ConfigHash:    configHash,
	}
}

// SaveMeta writes the metadata to disk.
func SaveMeta(path string, meta Meta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadMeta reads metadata from disk.
func LoadMeta(path string) (Meta, error) {
	var meta Meta
	data, err := os.ReadFile(path)
	if err != nil {
		return meta, err
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return meta, err
	}
	return meta, nil
}
