package format

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/memkit/repodex/internal/search"
)

// SearchOptions controls the grouped search output format.
type SearchOptions struct {
	NoFormat  bool
	WithScore bool
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
