package ts

import (
	"strings"
	"testing"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/textutil"
)

func TestChunkerGroupsConst(t *testing.T) {
	content := "const a = 1;\nconst b = 2;\nfunction foo() {}\n"
	cfg := config.ChunkingConfig{MaxLines: 50, OverlapLines: 5, MinChunkLines: 1}
	limits := config.LimitsConfig{MaxSnippetBytes: 200}

	chunks, err := ChunkFile("sample.ts", []byte(content), cfg, limits)
	if err != nil {
		t.Fatalf("chunk error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].StartLine != 1 || chunks[0].EndLine != 2 {
		t.Fatalf("unexpected const chunk range %d-%d", chunks[0].StartLine, chunks[0].EndLine)
	}
	if chunks[1].StartLine != 3 {
		t.Fatalf("unexpected function chunk start %d", chunks[1].StartLine)
	}
}

func TestChunkerExportBraceAppended(t *testing.T) {
	content := "function foo() {\n  return 1;\n}\nexport { foo };"
	cfg := config.ChunkingConfig{MaxLines: 50, OverlapLines: 5, MinChunkLines: 1}
	limits := config.LimitsConfig{MaxSnippetBytes: 200}

	chunks, err := ChunkFile("sample.ts", []byte(content), cfg, limits)
	if err != nil {
		t.Fatalf("chunk error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].EndLine != 4 {
		t.Fatalf("expected export line appended, end line %d", chunks[0].EndLine)
	}
}

func TestChunkerImportGrouping(t *testing.T) {
	content := "import a from 'a';\nimport b from 'b';\nconst c = 1;\n"
	cfg := config.ChunkingConfig{MaxLines: 50, OverlapLines: 5, MinChunkLines: 1}
	limits := config.LimitsConfig{MaxSnippetBytes: 200}

	chunks, err := ChunkFile("sample.ts", []byte(content), cfg, limits)
	if err != nil {
		t.Fatalf("chunk error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].StartLine != 1 || chunks[0].EndLine != 2 {
		t.Fatalf("unexpected import chunk lines %d-%d", chunks[0].StartLine, chunks[0].EndLine)
	}
}

func TestChunkerMaxLinesSplit(t *testing.T) {
	builder := strings.Builder{}
	builder.WriteString("function foo() {\n")
	for i := 0; i < 13; i++ {
		builder.WriteString("  const x = 1;\n")
	}
	builder.WriteString("}\n")
	lines := builder.String()
	cfg := config.ChunkingConfig{MaxLines: 5, OverlapLines: 1, MinChunkLines: 1}
	limits := config.LimitsConfig{MaxSnippetBytes: 200}

	chunks, err := ChunkFile("sample.ts", []byte(lines), cfg, limits)
	if err != nil {
		t.Fatalf("chunk error: %v", err)
	}
	if len(chunks) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(chunks))
	}
	if chunks[1].StartLine != 5 || chunks[1].EndLine != 9 {
		t.Fatalf("unexpected second chunk range %d-%d", chunks[1].StartLine, chunks[1].EndLine)
	}
	if chunks[2].StartLine != 9 {
		t.Fatalf("expected overlap start at 9, got %d", chunks[2].StartLine)
	}
}

func TestChunkerFallbackNoTriggers(t *testing.T) {
	content := "// comment only\nconsole.log('x')\n"
	cfg := config.ChunkingConfig{MaxLines: 50, OverlapLines: 5, MinChunkLines: 1}
	limits := config.LimitsConfig{MaxSnippetBytes: 200}

	chunks, err := ChunkFile("sample.ts", []byte(content), cfg, limits)
	if err != nil {
		t.Fatalf("chunk error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatalf("expected at least one chunk")
	}
	expectedEnd := uint32(len(strings.Split(content, "\n")))
	if chunks[0].StartLine != 1 || chunks[0].EndLine != expectedEnd {
		t.Fatalf("unexpected chunk range %d-%d", chunks[0].StartLine, chunks[0].EndLine)
	}
}

func TestChunkerNormalizesCRLF(t *testing.T) {
	content := "import a from 'a';\r\n\r\nfunction foo() {\r\n  return 1;\r\n}\r\n"
	cfg := config.ChunkingConfig{MaxLines: 50, OverlapLines: 5, MinChunkLines: 1}
	limits := config.LimitsConfig{MaxSnippetBytes: 200}

	chunks, err := ChunkFile("sample.ts", []byte(content), cfg, limits)
	if err != nil {
		t.Fatalf("chunk error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	expectedEnd := uint32(len(strings.Split(textutil.NormalizeNewlinesString(content), "\n")))
	if chunks[0].StartLine != 1 || chunks[0].EndLine != 2 {
		t.Fatalf("unexpected import chunk range %d-%d", chunks[0].StartLine, chunks[0].EndLine)
	}
	if chunks[1].EndLine != expectedEnd {
		t.Fatalf("unexpected function chunk end %d", chunks[1].EndLine)
	}
}

func TestChunkerIterativeMinMerge(t *testing.T) {
	content := "function a() {}\nfunction b() {}\nfunction c() {}"
	cfg := config.ChunkingConfig{MaxLines: 50, OverlapLines: 5, MinChunkLines: 3}
	limits := config.LimitsConfig{MaxSnippetBytes: 200}

	chunks, err := ChunkFile("sample.ts", []byte(content), cfg, limits)
	if err != nil {
		t.Fatalf("chunk error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 merged chunk, got %d", len(chunks))
	}
	if chunks[0].StartLine != 1 || chunks[0].EndLine != 3 {
		t.Fatalf("unexpected merged chunk range %d-%d", chunks[0].StartLine, chunks[0].EndLine)
	}
}

func TestUpdateDepthNestedAndComments(t *testing.T) {
	braceDepth := 0
	parenDepth := 0
	inBlockComment := false

	steps := []struct {
		line      string
		wantBrace int
		wantParen int
		wantBlock bool
	}{
		{"if (foo", 0, 1, false},
		{"  && (bar)) {", 1, 0, false},
		{"  /* comment with { (", 1, 0, true},
		{"  still comment */ })", 0, 0, false},
		{"}", 0, 0, false},
	}

	for idx, step := range steps {
		updateDepth(step.line, &braceDepth, &parenDepth, &inBlockComment)
		if braceDepth != step.wantBrace || parenDepth != step.wantParen || inBlockComment != step.wantBlock {
			t.Fatalf("step %d after line %q got brace=%d paren=%d block=%v; want brace=%d paren=%d block=%v",
				idx, step.line, braceDepth, parenDepth, inBlockComment, step.wantBrace, step.wantParen, step.wantBlock)
		}
	}
}
