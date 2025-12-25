package index

import (
	"path/filepath"
	"sort"
)

// PrecomputedFile holds chunk/token data ready for index assembly.
type PrecomputedFile struct {
	Path   string
	MTime  int64
	Size   int64
	Hash64 uint64
	Chunks []PrecomputedChunk
}

// PrecomputedChunk describes a chunk with its tokens.
type PrecomputedChunk struct {
	StartLine uint32
	EndLine   uint32
	Snippet   string
	Tokens    []string
}

// BuildFromPrecomputed assembles index structures from precomputed chunks/tokens.
func BuildFromPrecomputed(files []PrecomputedFile) ([]FileEntry, []ChunkEntry, map[string][]uint32, error) {
	sortedFiles := make([]PrecomputedFile, len(files))
	copy(sortedFiles, files)
	sort.Slice(sortedFiles, func(i, j int) bool {
		return filepath.ToSlash(sortedFiles[i].Path) < filepath.ToSlash(sortedFiles[j].Path)
	})

	var (
		fileEntries  []FileEntry
		chunkEntries []ChunkEntry
		postings            = make(map[string][]uint32)
		nextFileID   uint32 = 1
		nextChunkID  uint32 = 1
	)

	for _, f := range sortedFiles {
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

		for _, ch := range f.Chunks {
			chunkEntry := ChunkEntry{
				ChunkID:   nextChunkID,
				FileID:    fileEntry.FileID,
				Path:      path,
				StartLine: ch.StartLine,
				EndLine:   ch.EndLine,
				Snippet:   ch.Snippet,
			}
			unique := make(map[string]struct{})
			for _, t := range ch.Tokens {
				unique[t] = struct{}{}
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
