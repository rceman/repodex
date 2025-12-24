package hash

import (
	"bufio"
	"io"
)

const (
	offset64 = 14695981039346656037
	prime64  = 1099511628211
)

// Sum64 computes FNV-1a 64-bit hash for the provided bytes.
func Sum64(data []byte) uint64 {
	var h uint64 = offset64
	for _, b := range data {
		h ^= uint64(b)
		h *= prime64
	}
	return h
}

// Sum64Reader computes the hash over all bytes from the reader.
func Sum64Reader(r io.Reader) (uint64, error) {
	var h uint64 = offset64
	buf := bufio.NewReader(r)
	for {
		b, err := buf.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		h ^= uint64(b)
		h *= prime64
	}
	return h, nil
}
