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
	NoFormat  bool
	WithScore bool
	Explain   bool
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
	width := 1
	if maxLine > 0 {
		width = len(strconv.FormatUint(uint64(maxLine), 10))
	}

	writer := bufio.NewWriter(w)
	lastPath := ""

	for _, hit := range results {
		if hit.Path != lastPath {
			if _, err := fmt.Fprintf(writer, "-%s\n", hit.Path); err != nil {
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
		if opt.NoFormat {
			codeLine = escapeNoFormatLine(codeLine)
		}
		if _, err := fmt.Fprintf(writer, " [%*d] %s", width, line, codeLine); err != nil {
			return err
		}
		if opt.Explain && len(hit.Why) > 0 {
			why := append([]string(nil), hit.Why...)
			if !sort.StringsAreSorted(why) {
				sort.Strings(why)
			}
			if _, err := fmt.Fprintf(writer, "  [why: %s]", strings.Join(why, ",")); err != nil {
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
