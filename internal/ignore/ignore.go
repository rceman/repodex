package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

var defaultDirs = []string{
	"node_modules/",
	"dist/",
	"build/",
	".next/",
	"coverage/",
	".git/",
	"out/",
}

// WriteDefaultIgnore writes the default ignore file.
func WriteDefaultIgnore(path string) error {
	builder := strings.Builder{}
	for i, dir := range defaultDirs {
		builder.WriteString(dir)
		if i != len(defaultDirs)-1 {
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
