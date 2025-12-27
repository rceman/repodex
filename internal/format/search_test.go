package format

import (
	"bytes"
	"strings"
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

func TestWriteSearchCompactOutput(t *testing.T) {
	results := []search.Result{
		{
			Path:      "b.go",
			StartLine: 120,
			Snippet:   "  func runSearch() error {\n  return nil\n}",
			Why:       []string{"run", "search"},
		},
		{
			Path:      "b.go",
			StartLine: 5,
			MatchLine: 7,
			Snippet:   "  -dash\nnext",
		},
		{
			Path:      "a.go",
			StartLine: 42,
			Snippet:   "   alpha",
		},
	}

	var buf bytes.Buffer
	if err := WriteSearchCompact(&buf, results, SearchOptions{}); err != nil {
		t.Fatalf("format error: %v", err)
	}

	want := "" +
		"-b.go\n" +
		" [120] func runSearch() error {\n" +
		" [  7] -dash\n" +
		"-a.go\n" +
		" [ 42] alpha\n"

	got := buf.String()
	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestWriteSearchCompactExplainNoFormat(t *testing.T) {
	results := []search.Result{
		{
			Path:      "file.go",
			StartLine: 10,
			Snippet:   "  @line\nnext",
			Why:       []string{"beta", "alpha"},
		},
	}

	var buf bytes.Buffer
	if err := WriteSearchCompact(&buf, results, SearchOptions{NoFormat: true, Explain: true}); err != nil {
		t.Fatalf("format error: %v", err)
	}

	want := "" +
		"-file.go\n" +
		" [10] \\@line  [why: alpha,beta]\n"

	got := buf.String()
	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestWriteSearchCompactScopeOutput(t *testing.T) {
	results := []search.Result{
		{
			Path:           "file.go",
			StartLine:      10,
			MatchLine:      10,
			ScopeStartLine: 10,
			ScopeEndLine:   20,
			Snippet:        "  func run() {}",
		},
		{
			Path:           "file.go",
			StartLine:      15,
			MatchLine:      15,
			ScopeStartLine: 10,
			ScopeEndLine:   20,
			Snippet:        "  run()",
		},
		{
			Path:      "file.go",
			StartLine: 30,
			Snippet:   "  outside()",
		},
	}

	var buf bytes.Buffer
	if err := WriteSearchCompact(&buf, results, SearchOptions{Scope: true}); err != nil {
		t.Fatalf("format error: %v", err)
	}

	want := "" +
		"-file.go\n" +
		"10-20: func run() {}\n" +
		"10-20@15: run()\n" +
		"30: outside()\n"

	got := buf.String()
	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestWriteSearchCompactNoAnsiWhenDisabled(t *testing.T) {
	results := []search.Result{
		{
			Path:      "file.go",
			StartLine: 3,
			Snippet:   "  func run() {}",
			Why:       []string{"run"},
		},
	}

	var buf bytes.Buffer
	if err := WriteSearchCompact(&buf, results, SearchOptions{}); err != nil {
		t.Fatalf("format error: %v", err)
	}

	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatalf("unexpected ansi output")
	}
}

func TestWriteSearchCompactAnsiOutput(t *testing.T) {
	results := []search.Result{
		{
			Path:      "file.go",
			StartLine: 3,
			Snippet:   "  func run() {}",
			Why:       []string{"run"},
		},
	}

	var buf bytes.Buffer
	if err := WriteSearchCompact(&buf, results, SearchOptions{
		ColorPolicy: ColorPolicy{Enabled: true},
		Explain:     true,
	}); err != nil {
		t.Fatalf("format error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, ansiCyan) {
		t.Fatalf("expected path color")
	}
	if !strings.Contains(out, ansiDim) {
		t.Fatalf("expected dim line token or explain")
	}
	if !strings.Contains(out, ansiGreen) {
		t.Fatalf("expected definition color")
	}
	if !strings.Contains(out, ansiUnderline) {
		t.Fatalf("expected term highlight")
	}
}
