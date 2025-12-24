package scan

import (
	"os"

	"github.com/memkit/repodex/internal/hash"
)

// FileHash returns FNV-1a 64 hash of the file content.
func FileHash(path string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	return hash.Sum64Reader(file)
}
