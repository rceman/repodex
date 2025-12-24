package index

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/lang"
	"github.com/memkit/repodex/internal/scan"
)

// Build constructs in-memory index structures.
func Build(files []scan.ScannedFile, plugin lang.LanguagePlugin, cfg config.Config) ([]FileEntry, []ChunkEntry, map[string][]uint32, error) {
	var fileEntries []FileEntry
	var chunkEntries []ChunkEntry
	postings := make(map[string][]uint32)

	var nextFileID uint32 = 1
	var nextChunkID uint32 = 1

	for _, f := range files {
		path := filepath.ToSlash(f.Path)
		fileEntry := FileEntry{
			FileID: nextFileID,
			Path:   path,
			MTime:  f.MTime,
			Size:   f.Size,
			Hash64: f.Hash64,
		}
		nextFileID++
		fileEntries = append(fileEntries, fileEntry)

		chunks, err := plugin.ChunkFile(path, f.Content, cfg.Chunk, cfg.Limits)
		if err != nil {
			return nil, nil, nil, err
		}

		lines := strings.Split(string(f.Content), "\n")

		for _, ch := range chunks {
			chunkText := extractText(lines, int(ch.StartLine), int(ch.EndLine))
			tokens := plugin.TokenizeChunk(path, chunkText, cfg.Token)
			unique := make(map[string]struct{})
			for _, t := range tokens {
				unique[t] = struct{}{}
			}
			chunkEntry := ChunkEntry{
				ChunkID:   nextChunkID,
				FileID:    fileEntry.FileID,
				Path:      path,
				StartLine: ch.StartLine,
				EndLine:   ch.EndLine,
				Snippet:   ch.Snippet,
			}
			chunkEntries = append(chunkEntries, chunkEntry)
			for term := range unique {
				postings[term] = append(postings[term], chunkEntry.ChunkID)
			}
			nextChunkID++
		}
	}

	for term, ids := range postings {
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		postings[term] = dedupUint32(ids)
	}

	return fileEntries, chunkEntries, postings, nil
}

func extractText(lines []string, start, end int) string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return ""
	}
	return strings.Join(lines[start-1:end], "\n")
}

func dedupUint32(in []uint32) []uint32 {
	if len(in) == 0 {
		return in
	}
	out := []uint32{in[0]}
	for i := 1; i < len(in); i++ {
		if in[i] != in[i-1] {
			out = append(out, in[i])
		}
	}
	return out
}
