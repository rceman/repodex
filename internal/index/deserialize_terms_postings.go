package index

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

// TermInfo describes the location and frequency of a term.
type TermInfo struct {
	Offset uint64
	DF     uint32
}

// LoadTerms reads term metadata from terms.dat.
func LoadTerms(path string) (map[string]TermInfo, uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var termCount uint32
	if err := binary.Read(f, binary.LittleEndian, &termCount); err != nil {
		return nil, 0, err
	}

	terms := make(map[string]TermInfo, termCount)
	for i := uint32(0); i < termCount; i++ {
		term, err := readString(f)
		if err != nil {
			return nil, 0, err
		}
		var info TermInfo
		if err := binary.Read(f, binary.LittleEndian, &info.Offset); err != nil {
			return nil, 0, err
		}
		if err := binary.Read(f, binary.LittleEndian, &info.DF); err != nil {
			return nil, 0, err
		}
		terms[term] = info
	}
	return terms, termCount, nil
}

// LoadPostings reads postings as a uint32 slice.
func LoadPostings(path string) ([]uint32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("postings file size is invalid")
	}
	count := len(data) / 4
	postings := make([]uint32, count)
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, postings); err != nil {
		return nil, err
	}
	return postings, nil
}
