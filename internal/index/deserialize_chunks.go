package index

import (
	"encoding/binary"
	"os"
)

// LoadChunkEntries reads chunk data from chunks.dat.
func LoadChunkEntries(path string) ([]ChunkEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var count uint32
	if err := binary.Read(f, binary.LittleEndian, &count); err != nil {
		return nil, err
	}

	entries := make([]ChunkEntry, 0, count)
	for i := uint32(0); i < count; i++ {
		var ch ChunkEntry
		if err := binary.Read(f, binary.LittleEndian, &ch.ChunkID); err != nil {
			return nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &ch.FileID); err != nil {
			return nil, err
		}
		pathStr, err := readString(f)
		if err != nil {
			return nil, err
		}
		ch.Path = pathStr
		if err := binary.Read(f, binary.LittleEndian, &ch.StartLine); err != nil {
			return nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &ch.EndLine); err != nil {
			return nil, err
		}
		snippet, err := readString(f)
		if err != nil {
			return nil, err
		}
		ch.Snippet = snippet
		entries = append(entries, ch)
	}
	return entries, nil
}
