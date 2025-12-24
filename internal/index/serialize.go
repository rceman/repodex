package index

import (
	"encoding/binary"
	"io"
	"os"
	"sort"

	"github.com/memkit/repodex/internal/store"
)

// Serialize writes index data to disk.
func Serialize(root string, files []FileEntry, chunks []ChunkEntry, postings map[string][]uint32) error {
	if err := os.MkdirAll(store.Dir(root), 0o755); err != nil {
		return err
	}

	if err := writeFiles(store.FilesPath(root), files); err != nil {
		return err
	}
	if err := writeChunks(store.ChunksPath(root), chunks); err != nil {
		return err
	}
	if err := writeTermsAndPostings(store.TermsPath(root), store.PostingsPath(root), postings); err != nil {
		return err
	}
	return nil
}

func writeFiles(path string, files []FileEntry) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := binary.Write(f, binary.LittleEndian, uint32(len(files))); err != nil {
		return err
	}
	for _, fe := range files {
		if err := binary.Write(f, binary.LittleEndian, fe.FileID); err != nil {
			return err
		}
		if err := writeString(f, fe.Path); err != nil {
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, fe.MTime); err != nil {
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, fe.Size); err != nil {
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, fe.Hash64); err != nil {
			return err
		}
	}
	return nil
}

func writeChunks(path string, chunks []ChunkEntry) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := binary.Write(f, binary.LittleEndian, uint32(len(chunks))); err != nil {
		return err
	}
	for _, ch := range chunks {
		if err := binary.Write(f, binary.LittleEndian, ch.ChunkID); err != nil {
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, ch.FileID); err != nil {
			return err
		}
		if err := writeString(f, ch.Path); err != nil {
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, ch.StartLine); err != nil {
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, ch.EndLine); err != nil {
			return err
		}
		if err := writeString(f, ch.Snippet); err != nil {
			return err
		}
	}
	return nil
}

func writeTermsAndPostings(termsPath, postingsPath string, postings map[string][]uint32) error {
	postingsFile, err := os.Create(postingsPath)
	if err != nil {
		return err
	}
	defer postingsFile.Close()

	termsFile, err := os.Create(termsPath)
	if err != nil {
		return err
	}
	defer termsFile.Close()

	terms := make([]string, 0, len(postings))
	for term := range postings {
		terms = append(terms, term)
	}
	sort.Strings(terms)

	if err := binary.Write(termsFile, binary.LittleEndian, uint32(len(terms))); err != nil {
		return err
	}

	var offset uint64
	for _, term := range terms {
		ids := postings[term]
		if err := writeString(termsFile, term); err != nil {
			return err
		}
		if err := binary.Write(termsFile, binary.LittleEndian, offset); err != nil {
			return err
		}
		if err := binary.Write(termsFile, binary.LittleEndian, uint32(len(ids))); err != nil {
			return err
		}
		for _, id := range ids {
			if err := binary.Write(postingsFile, binary.LittleEndian, id); err != nil {
				return err
			}
			offset += 4
		}
	}
	return nil
}

func writeString(w io.Writer, s string) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(s))); err != nil {
		return err
	}
	_, err := io.WriteString(w, s)
	return err
}
