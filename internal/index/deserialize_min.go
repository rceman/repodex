package index

import (
	"encoding/binary"
	"io"
	"os"
)

// LoadFileEntries reads minimal file data from files.dat.
func LoadFileEntries(path string) ([]FileEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var count uint32
	if err := binary.Read(f, binary.LittleEndian, &count); err != nil {
		return nil, err
	}
	entries := make([]FileEntry, 0, count)
	for i := uint32(0); i < count; i++ {
		var fe FileEntry
		if err := binary.Read(f, binary.LittleEndian, &fe.FileID); err != nil {
			return nil, err
		}
		pathStr, err := readString(f)
		if err != nil {
			return nil, err
		}
		fe.Path = pathStr
		if err := binary.Read(f, binary.LittleEndian, &fe.MTime); err != nil {
			return nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &fe.Size); err != nil {
			return nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &fe.Hash64); err != nil {
			return nil, err
		}
		entries = append(entries, fe)
	}
	return entries, nil
}

func readString(r io.Reader) (string, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}
