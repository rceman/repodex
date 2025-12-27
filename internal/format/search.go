package format

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/memkit/repodex/internal/search"
)

// SearchOptions controls search output formatting.
type SearchOptions struct {
	NoFormat    bool
	WithScore   bool
	Explain     bool
	Scope       bool
	ColorPolicy ColorPolicy
	QueryTerms  []string
}

type group struct {
	tag       string
	fileOrder []string
	files     map[string][]search.Result
}

// WriteSearchGrouped writes Variant A grouped search output to the writer.
func WriteSearchGrouped(w io.Writer, results []search.Result, opt SearchOptions) error {
	groups := make(map[string]*group)
	var groupOrder []string

	ensureGroup := func(tag string) *group {
		g, ok := groups[tag]
		if ok {
			return g
		}
		g = &group{
			tag:   tag,
			files: make(map[string][]search.Result),
		}
		groups[tag] = g
		groupOrder = append(groupOrder, tag)
		return g
	}

	for _, r := range results {
		tags := r.Why
		if len(tags) == 0 {
			tags = []string{"none"}
		}
		for _, tag := range tags {
			g := ensureGroup(tag)
			if _, ok := g.files[r.Path]; !ok {
				g.fileOrder = append(g.fileOrder, r.Path)
			}
			g.files[r.Path] = append(g.files[r.Path], r)
		}
	}

	writer := bufio.NewWriter(w)
	for _, tag := range groupOrder {
		g := groups[tag]
		if _, err := fmt.Fprintf(writer, ">why=%s\n", g.tag); err != nil {
			return err
		}
		for _, path := range g.fileOrder {
			if _, err := fmt.Fprintf(writer, "-%s\n", path); err != nil {
				return err
			}
			for _, hit := range g.files[path] {
				if err := writeHit(writer, hit, opt); err != nil {
					return err
				}
			}
		}
	}
	return writer.Flush()
}

func writeHit(w *bufio.Writer, hit search.Result, opt SearchOptions) error {
	lines := strings.Split(hit.Snippet, "\n")
	trimmed := make([]string, len(lines))
	for i, line := range lines {
		trimmed[i] = strings.TrimLeft(line, " \t")
	}
	byteCount := len([]byte(strings.Join(trimmed, "\n")))
	prefix := " "
	if opt.NoFormat {
		prefix = ""
	}
	if opt.WithScore {
		if _, err := fmt.Fprintf(w, "%s@%d:%d-%d!%d~%.2f\n", prefix, hit.ChunkID, hit.StartLine, hit.EndLine, byteCount, hit.Score); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(w, "%s@%d:%d-%d!%d\n", prefix, hit.ChunkID, hit.StartLine, hit.EndLine, byteCount); err != nil {
			return err
		}
	}

	for _, line := range trimmed {
		outLine := line
		if opt.NoFormat {
			if len(outLine) > 0 {
				switch outLine[0] {
				case '>', '-', '@':
					outLine = "\\" + outLine
				}
			}
		} else {
			outLine = "  " + outLine
		}
		if _, err := w.WriteString(outLine); err != nil {
			return err
		}
		if _, err := w.WriteString("\n"); err != nil {
			return err
		}
	}
	return nil
}

// WriteSearchCompact writes a compact search output without why grouping.
func WriteSearchCompact(w io.Writer, results []search.Result, opt SearchOptions) error {
	if len(results) == 0 {
		return nil
	}

	width := 1
	if !opt.Scope {
		maxLine := uint32(0)
		for _, hit := range results {
			line := hit.MatchLine
			if line == 0 {
				line = hit.StartLine
			}
			if line > maxLine {
				maxLine = line
			}
		}
		if maxLine > 0 {
			width = len(strconv.FormatUint(uint64(maxLine), 10))
		}
	}

	writer := bufio.NewWriter(w)
	lastPath := ""
	colorEnabled := opt.ColorPolicy.Enabled && !opt.NoFormat

	for _, hit := range results {
		isTest := strings.HasSuffix(hit.Path, "_test.go")
		if hit.Path != lastPath {
			pathLine := "-" + hit.Path
			if colorEnabled {
				pathPrefix := ansiCyan + ansiBold
				if isTest {
					pathPrefix = ansiDim + ansiCyan
				}
				pathLine = ansiWrap(true, pathPrefix, pathLine, ansiReset)
			}
			if _, err := fmt.Fprintf(writer, "%s\n", pathLine); err != nil {
				return err
			}
			lastPath = hit.Path
		}

		line := hit.MatchLine
		if line == 0 {
			line = hit.StartLine
		}
		codeLine := hit.Snippet
		if idx := strings.IndexByte(codeLine, '\n'); idx >= 0 {
			codeLine = codeLine[:idx]
		}
		codeLine = strings.TrimLeft(codeLine, " \t")
		isDef := strings.HasPrefix(codeLine, "func ")
		isScopeDef := opt.Scope && hit.ScopeStartLine > 0 && line == hit.ScopeStartLine && isDef
		if opt.Scope {
			isDef = isScopeDef
		}
		if opt.NoFormat {
			codeLine = escapeNoFormatLine(codeLine)
		}
		if colorEnabled {
			terms := opt.QueryTerms
			if len(terms) == 0 {
				terms = hit.Why
			}
			codeLine = highlightTerms(codeLine, terms, colorEnabled)
			if isDef {
				codeLine = ansiWrap(true, ansiGreen, codeLine, ansiReset)
			} else if isTest {
				codeLine = ansiWrap(true, ansiDim, codeLine, ansiReset)
			}
		}
		if opt.Scope {
			token := ""
			if hit.ScopeStartLine > 0 && hit.ScopeEndLine > 0 {
				if isScopeDef {
					token = fmt.Sprintf("%d-%d:", hit.ScopeStartLine, hit.ScopeEndLine)
				} else {
					token = fmt.Sprintf("%d-%d@%d:", hit.ScopeStartLine, hit.ScopeEndLine, line)
				}
			} else {
				token = fmt.Sprintf("%d:", line)
			}
			if colorEnabled {
				token = ansiWrap(true, ansiDim, token, ansiReset)
			}
			if _, err := fmt.Fprintf(writer, "%s %s", token, codeLine); err != nil {
				return err
			}
		} else {
			lineToken := fmt.Sprintf("[%*d]", width, line)
			if colorEnabled {
				lineToken = ansiWrap(true, ansiDim, lineToken, ansiReset)
			}
			if _, err := fmt.Fprintf(writer, " %s %s", lineToken, codeLine); err != nil {
				return err
			}
		}
		if opt.Explain && len(hit.Why) > 0 {
			why := append([]string(nil), hit.Why...)
			if !sort.StringsAreSorted(why) {
				sort.Strings(why)
			}
			explain := fmt.Sprintf("  [why: %s]", strings.Join(why, ","))
			if colorEnabled {
				explain = ansiWrap(true, ansiDim, explain, ansiReset)
			}
			if _, err := writer.WriteString(explain); err != nil {
				return err
			}
		}
		if _, err := writer.WriteString("\n"); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func escapeNoFormatLine(line string) string {
	if len(line) == 0 {
		return line
	}
	switch line[0] {
	case '>', '-', '@':
		return "\\" + line
	default:
		return line
	}
}

type termMatch struct {
	start int
	end   int
}

func highlightTerms(line string, terms []string, enabled bool) string {
	if !enabled || line == "" || len(terms) == 0 {
		return line
	}
	matches := collectTermMatches(line, terms)
	if len(matches) == 0 {
		return line
	}
	var b strings.Builder
	b.Grow(len(line) + len(matches)*len(ansiUnderline)*2)
	last := 0
	for _, m := range matches {
		if m.start < last {
			continue
		}
		b.WriteString(line[last:m.start])
		b.WriteString(ansiUnderline)
		b.WriteString(line[m.start:m.end])
		b.WriteString(ansiUnderlineOff)
		last = m.end
	}
	b.WriteString(line[last:])
	return b.String()
}

func collectTermMatches(line string, terms []string) []termMatch {
	seen := make(map[string]struct{}, len(terms))
	matches := make([]termMatch, 0, len(terms))
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		ident := isIdentifierTerm(term)
		start := 0
		for {
			idx := strings.Index(line[start:], term)
			if idx < 0 {
				break
			}
			idx += start
			end := idx + len(term)
			if ident && !isIdentifierBoundary(line, idx, end) {
				start = end
				continue
			}
			matches = append(matches, termMatch{start: idx, end: end})
			start = end
		}
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].start != matches[j].start {
			return matches[i].start < matches[j].start
		}
		return matches[i].end > matches[j].end
	})
	filtered := matches[:0]
	lastEnd := -1
	for _, m := range matches {
		if m.start < lastEnd {
			continue
		}
		filtered = append(filtered, m)
		lastEnd = m.end
	}
	return filtered
}

func isIdentifierTerm(term string) bool {
	if term == "" {
		return false
	}
	for i := 0; i < len(term); i++ {
		if !isIdentifierChar(term[i]) {
			return false
		}
	}
	return true
}

func isIdentifierBoundary(line string, start, end int) bool {
	if start > 0 && isIdentifierChar(line[start-1]) {
		return false
	}
	if end < len(line) && isIdentifierChar(line[end]) {
		return false
	}
	return true
}

func isIdentifierChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '_':
		return true
	default:
		return false
	}
}
