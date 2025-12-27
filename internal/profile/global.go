package profile

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var knownBinaryExts = map[string]struct{}{
	".zip":   {},
	".gz":    {},
	".bz2":   {},
	".xz":    {},
	".7z":    {},
	".rar":   {},
	".tar":   {},
	".tgz":   {},
	".png":   {},
	".jpg":   {},
	".jpeg":  {},
	".webp":  {},
	".gif":   {},
	".ico":   {},
	".bmp":   {},
	".tiff":  {},
	".psd":   {},
	".ai":    {},
	".mp4":   {},
	".mov":   {},
	".mkv":   {},
	".webm":  {},
	".avi":   {},
	".mp3":   {},
	".wav":   {},
	".flac":  {},
	".ogg":   {},
	".woff":  {},
	".woff2": {},
	".ttf":   {},
	".otf":   {},
	".eot":   {},
	".pdf":   {},
	".doc":   {},
	".docx":  {},
	".xls":   {},
	".xlsx":  {},
	".ppt":   {},
	".pptx":  {},
	".wasm":  {},
	".exe":   {},
	".dll":   {},
	".so":    {},
	".dylib": {},
	".class": {},
	".jar":   {},
	".bin":   {},
	".dat":   {},
}

var knownBinarySuffixes = map[string]struct{}{
	".tar.gz":  {},
	".tar.bz2": {},
	".tar.xz":  {},
}

// GlobalScanIgnore returns default scan ignores.
func GlobalScanIgnore(hasPackageJSON bool) []string {
	patterns := []string{
		"**/*.svg",
		".git/",
		".repodex/",
	}
	if hasPackageJSON {
		patterns = append(patterns, "package-lock.json")
	}
	return patterns
}

// IsKnownBinaryExt reports whether the path matches a known binary extension or suffix.
func IsKnownBinaryExt(lowerPath string) bool {
	for suffix := range knownBinarySuffixes {
		if strings.HasSuffix(lowerPath, suffix) {
			return true
		}
	}
	ext := filepath.Ext(lowerPath)
	_, ok := knownBinaryExts[ext]
	return ok
}

// IsBinarySniff performs a simple binary check by reading a sample.
func IsBinarySniff(path string, sampleSize int) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	buf := make([]byte, sampleSize)
	n, err := io.ReadFull(file, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}
	buf = buf[:n]

	if bytes.IndexByte(buf, 0) >= 0 {
		return true, nil
	}
	if !utf8.Valid(buf) {
		return true, nil
	}
	return false, nil
}
