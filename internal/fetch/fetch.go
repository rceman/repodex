package fetch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/memkit/repodex/internal/index"
	"github.com/memkit/repodex/internal/store"
)

// Request describes a fetch request.
type Request struct {
	IDs      []uint32 `json:"ids"`
	MaxLines int      `json:"max_lines"`
}

// ChunkText contains extracted lines for a chunk.
type ChunkText struct {
	ChunkID      uint32   `json:"chunk_id"`
	Path         string   `json:"path"`
	StartLine    uint32   `json:"start_line"`
	EndLine      uint32   `json:"end_line"`
	ReturnedFrom uint32   `json:"returned_from"`
	ReturnedTo   uint32   `json:"returned_to"`
	Lines        []string `json:"lines"`
}

// Fetch returns chunk text constrained by limits.
func Fetch(root string, ids []uint32, maxLines int) ([]ChunkText, error) {
	if len(ids) > 5 {
		ids = ids[:5]
	}
	if maxLines <= 0 || maxLines > 120 {
		maxLines = 120
	}

	chunks, err := index.LoadChunkEntries(store.ChunksPath(root))
	if err != nil {
		return nil, err
	}
	chunkMap := make(map[uint32]index.ChunkEntry, len(chunks))
	for _, ch := range chunks {
		chunkMap[ch.ChunkID] = ch
	}

	var results []ChunkText
	for _, id := range ids {
		ch, ok := chunkMap[id]
		if !ok {
			return nil, fmt.Errorf("chunk %d not found in index", id)
		}
		fullPath := filepath.Join(root, ch.Path)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("chunk %d path %s: %w", id, ch.Path, err)
		}
		lines := strings.Split(string(data), "\n")
		start := int(ch.StartLine)
		end := int(ch.EndLine)
		if start < 1 {
			start = 1
		}
		if end > len(lines) {
			end = len(lines)
		}
		if start > end {
			start = end
		}
		returnedTo := end
		if (end - start + 1) > maxLines {
			returnedTo = start + maxLines - 1
		}
		if returnedTo > end {
			returnedTo = end
		}

		var formatted []string
		for i := start; i <= returnedTo; i++ {
			lineText := ""
			if i-1 < len(lines) {
				lineText = lines[i-1]
			}
			formatted = append(formatted, fmt.Sprintf("%d| %s", i, lineText))
		}

		results = append(results, ChunkText{
			ChunkID:      ch.ChunkID,
			Path:         ch.Path,
			StartLine:    ch.StartLine,
			EndLine:      ch.EndLine,
			ReturnedFrom: uint32(start),
			ReturnedTo:   uint32(returnedTo),
			Lines:        formatted,
		})
	}

	return results, nil
}
