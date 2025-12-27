package format

import (
	"bytes"
	"testing"

	"github.com/memkit/repodex/internal/search"
)

func TestWriteSearchGroupedOrderingAndBytes(t *testing.T) {
	results := []search.Result{
		{
			ChunkID:   1,
			Path:      "b.go",
			StartLine: 1,
			EndLine:   2,
			Snippet:   "alpha\nbeta",
			Why:       []string{"profile", "banana"},
		},
		{
			ChunkID:   2,
			Path:      "b.go",
			StartLine: 3,
			EndLine:   4,
			Snippet:   "   -dash",
		},
		{
			ChunkID:   3,
			Path:      "a.go",
			StartLine: 5,
			EndLine:   6,
			Snippet:   "@mark",
			Why:       []string{"banana"},
		},
	}

	var buf bytes.Buffer
	if err := WriteSearchGrouped(&buf, results, SearchOptions{}); err != nil {
		t.Fatalf("format error: %v", err)
	}

	want := "" +
		">why=profile\n" +
		"-b.go\n" +
		" @1:1-2!10\n" +
		"  alpha\n" +
		"  beta\n" +
		">why=banana\n" +
		"-b.go\n" +
		" @1:1-2!10\n" +
		"  alpha\n" +
		"  beta\n" +
		"-a.go\n" +
		" @3:5-6!5\n" +
		"  @mark\n" +
		">why=none\n" +
		"-b.go\n" +
		" @2:3-4!5\n" +
		"  -dash\n"

	got := buf.String()
	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestWriteSearchGroupedNoFormatEscapes(t *testing.T) {
	results := []search.Result{
		{
			ChunkID:   7,
			Path:      "file.go",
			StartLine: 10,
			EndLine:   12,
			Snippet:   "  @line\n\t-normal\n >header",
			Why:       []string{"why"},
		},
	}

	var buf bytes.Buffer
	if err := WriteSearchGrouped(&buf, results, SearchOptions{NoFormat: true}); err != nil {
		t.Fatalf("format error: %v", err)
	}

	want := "" +
		">why=why\n" +
		"-file.go\n" +
		"@7:10-12!21\n" +
		"\\@line\n" +
		"\\-normal\n" +
		"\\>header\n"

	got := buf.String()
	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}
