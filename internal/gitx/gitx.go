package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func runGit(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Keep the original error type via %w, but include output for debugging.
		return out, fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func isGitUnavailable(err error) bool {
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return true
	}
	return false
}

// TopLevel returns the absolute path to the git repository root for the provided
// starting directory.
func TopLevel(root string) (string, error) {
	out, err := runGit(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("git rev-parse --show-toplevel returned empty path")
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		path = abs
	}
	return path, nil
}

// IsRepo reports whether the provided root is inside a git work tree.
func IsRepo(root string) (bool, error) {
	out, err := runGit(root, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		if isGitUnavailable(err) {
			return false, nil
		}
		// If git is present but this is not a repo, rev-parse exits non-zero.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

// Head returns the SHA of HEAD at the provided root.
func Head(root string) (string, error) {
	out, err := runGit(root, "rev-parse", "HEAD")
	if err != nil {
		if isGitUnavailable(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// IsWorkTreeClean reports whether there are no staged or unstaged changes.
func IsWorkTreeClean(root string) (bool, error) {
	out, err := runGit(root, "status", "--porcelain")
	if err != nil {
		if isGitUnavailable(err) {
			return false, nil
		}
		return false, err
	}
	return len(bytes.TrimSpace(out)) == 0, nil
}

// StatusChangedPaths returns paths reported by `git status --porcelain`.
// This is intended for diagnostics (e.g., "repo is dirty only due to .repodex changes").
func StatusChangedPaths(root string) ([]string, error) {
	out, err := runGit(root, "status", "--porcelain")
	if err != nil {
		if isGitUnavailable(err) {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if len(line) < 4 {
			continue
		}
		// Porcelain v1 line format: XY <path> or XY <from> -> <to>
		rest := strings.TrimSpace(line[2:])
		if rest == "" {
			continue
		}
		// Handle rename: "old -> new"
		if i := strings.LastIndex(rest, "->"); i >= 0 {
			rest = strings.TrimSpace(rest[i+2:])
		}
		// Git prints paths with forward slashes.
		paths = append(paths, rest)
	}
	return paths, nil
}

// DiffNameOnly returns the set of paths changed between two commits/refs.
func DiffNameOnly(root, a, b string) ([]string, error) {
	out, err := runGit(root, "diff", "--name-only", a, b)
	if err != nil {
		if isGitUnavailable(err) {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var paths []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}
	return paths, nil
}
