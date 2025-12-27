package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// WriteIgnore writes ignore patterns to the provided path.
func WriteIgnore(path string, patterns []string) error {
	builder := strings.Builder{}
	for i, pattern := range patterns {
		builder.WriteString(pattern)
		if i != len(patterns)-1 {
			builder.WriteByte('\n')
		}
	}
	builder.WriteByte('\n')
	return os.WriteFile(path, []byte(builder.String()), 0o644)
}

// LoadDirs returns ignore directories from a file, trimming trailing slashes.
func LoadDirs(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var dirs []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSuffix(line, "/")
		if line != "" {
			dirs = append(dirs, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return dirs, nil
}

// IsIgnoredDir reports whether the path should be skipped based on ignore lists.
func IsIgnoredDir(path string, ignoreDirs []string) bool {
	for _, dir := range ignoreDirs {
		if strings.HasPrefix(path, dir+"/") || strings.Contains(path, "/"+dir+"/") || path == dir {
			return true
		}
	}
	return false
}

// NormalizePath converts platform-specific separators to forward slashes for matching.
func NormalizePath(path string) string {
	return filepath.ToSlash(path)
}
