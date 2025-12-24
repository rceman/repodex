package index

// FileEntry describes a scanned file.
type FileEntry struct {
	FileID uint32
	Path   string
	MTime  int64
	Size   int64
	Hash64 uint64
}

// ChunkEntry captures a chunk of a file.
type ChunkEntry struct {
	ChunkID   uint32
	FileID    uint32
	Path      string
	StartLine uint32
	EndLine   uint32
	Snippet   string
}
