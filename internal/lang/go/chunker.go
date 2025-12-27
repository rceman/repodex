package golang

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/lang"
	"github.com/memkit/repodex/internal/textutil"
)

type block struct {
	start int
	end   int
}

// ChunkFile splits a Go file into chunk drafts using simple heuristics.
func ChunkFile(path string, content []byte, cfg config.ChunkingConfig, limits config.LimitsConfig) ([]lang.ChunkDraft, error) {
	normalized := textutil.NormalizeNewlinesString(string(content))
	lines := strings.Split(normalized, "\n")

	var blocks []block
	if strings.EqualFold(filepath.Ext(path), ".go") {
		blocks = collectBlocks(lines)
	}
	if len(blocks) == 0 && len(lines) > 0 {
		blocks = []block{{start: 1, end: len(lines)}}
	}
	blocks = enforceMinLines(blocks, cfg.MinChunkLines)

	var drafts []lang.ChunkDraft
	for _, b := range blocks {
		chunks := splitBlock(b, cfg.MaxLines, cfg.OverlapLines)
		for _, c := range chunks {
			snip := buildSnippet(lines, c.start, c.end, limits.MaxSnippetBytes)
			drafts = append(drafts, lang.ChunkDraft{
				StartLine: uint32(c.start),
				EndLine:   uint32(c.end),
				Snippet:   snip,
			})
		}
	}
	return drafts, nil
}

func collectBlocks(lines []string) []block {
	var blocks []block
	var braceDepth, parenDepth int
	inBlockComment := false
	currentType := ""
	currentIdx := -1

	for i, raw := range lines {
		lineNum := i + 1
		topLevel := braceDepth == 0 && parenDepth == 0 && !inBlockComment
		trimmed := strings.TrimSpace(raw)

		if topLevel {
			if isBoundary(trimmed) {
				trigger := classifyTrigger(trimmed)
				switch trigger {
				case "import":
					if currentType != "import" {
						blocks = append(blocks, block{start: lineNum, end: lineNum})
						currentIdx = len(blocks) - 1
						currentType = "import"
					} else {
						blocks[currentIdx].end = lineNum
					}
				case "constvar":
					if currentType != "constvar" {
						blocks = append(blocks, block{start: lineNum, end: lineNum})
						currentIdx = len(blocks) - 1
						currentType = "constvar"
					} else {
						blocks[currentIdx].end = lineNum
					}
				default:
					blocks = append(blocks, block{start: lineNum, end: lineNum})
					currentIdx = len(blocks) - 1
					currentType = trigger
				}
			} else if currentIdx >= 0 {
				blocks[currentIdx].end = lineNum
			}
		} else if currentIdx >= 0 {
			blocks[currentIdx].end = lineNum
		}

		updateDepth(raw, &braceDepth, &parenDepth, &inBlockComment)
	}
	return blocks
}

func isBoundary(trimmed string) bool {
	lowered := strings.ToLower(trimmed)
	prefixes := []string{
		"package ",
		"import ",
		"import(",
		"import (",
		"const ",
		"const(",
		"const (",
		"var ",
		"var(",
		"var (",
		"type ",
		"type(",
		"type (",
		"func ",
		"func(",
		"func (",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lowered, p) {
			return true
		}
	}
	return false
}

func classifyTrigger(trimmed string) string {
	lowered := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(lowered, "package "):
		return "package"
	case strings.HasPrefix(lowered, "import ") || strings.HasPrefix(lowered, "import(") || strings.HasPrefix(lowered, "import ("):
		return "import"
	case strings.HasPrefix(lowered, "const ") || strings.HasPrefix(lowered, "const(") || strings.HasPrefix(lowered, "const ("):
		return "constvar"
	case strings.HasPrefix(lowered, "var ") || strings.HasPrefix(lowered, "var(") || strings.HasPrefix(lowered, "var ("):
		return "constvar"
	case strings.HasPrefix(lowered, "type ") || strings.HasPrefix(lowered, "type(") || strings.HasPrefix(lowered, "type ("):
		return "type"
	case strings.HasPrefix(lowered, "func ") || strings.HasPrefix(lowered, "func(") || strings.HasPrefix(lowered, "func ("):
		return "func"
	default:
		return "other"
	}
}

func updateDepth(line string, braceDepth *int, parenDepth *int, inBlockComment *bool) {
	inString := false
	var stringDelim rune

	for i := 0; i < len(line); i++ {
		ch := rune(line[i])

		if *inBlockComment {
			if ch == '*' && i+1 < len(line) && rune(line[i+1]) == '/' {
				*inBlockComment = false
				i++
			}
			continue
		}

		if !inString && ch == '/' && i+1 < len(line) {
			next := rune(line[i+1])
			if next == '/' {
				break
			}
			if next == '*' {
				*inBlockComment = true
				i++
				continue
			}
		}

		if ch == '"' || ch == '\'' || ch == '`' {
			if inString && ch == stringDelim {
				inString = false
			} else if !inString {
				inString = true
				stringDelim = ch
			}
			continue
		}

		if inString {
			continue
		}

		switch ch {
		case '{':
			(*braceDepth)++
		case '}':
			if *braceDepth > 0 {
				(*braceDepth)--
			}
		case '(':
			(*parenDepth)++
		case ')':
			if *parenDepth > 0 {
				(*parenDepth)--
			}
		}
	}
}

func enforceMinLines(blocks []block, minLines int) []block {
	if len(blocks) == 0 {
		return blocks
	}
	var merged []block
	i := 0
	for i < len(blocks) {
		acc := blocks[i]
		length := acc.end - acc.start + 1
		j := i + 1
		for length < minLines && j < len(blocks) {
			acc.end = blocks[j].end
			length = acc.end - acc.start + 1
			j++
		}
		merged = append(merged, acc)
		i = j
	}
	return merged
}

func splitBlock(b block, maxLines int, overlap int) []block {
	if maxLines <= 0 || b.end <= b.start {
		return []block{b}
	}
	var chunks []block
	start := b.start
	for start <= b.end {
		end := start + maxLines - 1
		if end > b.end {
			end = b.end
		}
		chunks = append(chunks, block{start: start, end: end})
		if end == b.end {
			break
		}
		nextStart := end - overlap + 1
		if nextStart <= start {
			nextStart = end + 1
		}
		start = nextStart
	}
	return chunks
}

func buildSnippet(lines []string, start, end int, maxBytes int) string {
	var picked []string
	for i := start - 1; i < end && len(picked) < 3; i++ {
		if i < 0 || i >= len(lines) {
			break
		}
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		picked = append(picked, line)
	}
	snippet := strings.Join(picked, "\n")
	if maxBytes > 0 {
		b := []byte(snippet)
		if len(b) > maxBytes {
			b = b[:maxBytes]
			for len(b) > 0 && !utf8.Valid(b) {
				b = b[:len(b)-1]
			}
			snippet = string(b)
		}
	}
	return snippet
}
