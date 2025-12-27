package scan_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/index"
	"github.com/memkit/repodex/internal/lang/ts"
	"github.com/memkit/repodex/internal/profile"
	"github.com/memkit/repodex/internal/scan"
)

func TestIndexingIsDeterministicAcrossRuns(t *testing.T) {
	root := t.TempDir()
	fixtures := map[string]string{
		"b.ts":          "import './a'\nconst b = 1\nconst c = 2\n",
		"a.ts":          "export function a() { return 1 }\nexport const x = 2\n",
		"nested/c.ts":   "interface Foo { bar: string }\nconst c = 3\n",
		"bundle.js.map": "{}",
	}
	for path, content := range fixtures {
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	cfg := config.DefaultRuntimeConfig()
	cfg.Chunk = config.ChunkingConfig{MaxLines: 1, OverlapLines: 0, MinChunkLines: 1}
	cfg.Token = config.TokenizationConfig{
		MinTokenLen:            1,
		MaxTokenLen:            64,
		DropHexLen:             16,
		TokenizeStringLiterals: true,
		MaxFileBytesCode:       1024,
	}
	cfg.Scan = config.ScanConfig{MaxTextFileSizeBytes: 2048}
	cfg.Limits = config.LimitsConfig{MaxSnippetBytes: 200}

	rules, err := profile.BuildEffectiveRules(root, []string{"ts_js", "node"}, cfg)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	cfg.Token = rules.TokenConfig

	firstScan, err := scan.Walk(root, rules)
	if err != nil {
		t.Fatalf("first walk: %v", err)
	}
	for _, f := range firstScan {
		if strings.HasSuffix(strings.ToLower(f.Path), ".map") {
			t.Fatalf("unexpected map artifact indexed: %s", f.Path)
		}
	}
	secondScan, err := scan.Walk(root, rules)
	if err != nil {
		t.Fatalf("second walk: %v", err)
	}
	if !reflect.DeepEqual(firstScan, secondScan) {
		t.Fatalf("expected sorted walk results to match across runs")
	}

	shuffled := append([]scan.ScannedFile(nil), secondScan...)
	rand.New(rand.NewSource(42)).Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	plugin := ts.TSPlugin{}
	files1, chunks1, postings1, err := index.Build(firstScan, plugin, cfg)
	if err != nil {
		t.Fatalf("build from first scan: %v", err)
	}
	files2, chunks2, postings2, err := index.Build(shuffled, plugin, cfg)
	if err != nil {
		t.Fatalf("build from shuffled scan: %v", err)
	}

	if !reflect.DeepEqual(files1, files2) {
		t.Fatalf("file entries differ between runs:\n%v\n%v", files1, files2)
	}
	if !reflect.DeepEqual(chunks1, chunks2) {
		t.Fatalf("chunk entries differ between runs:\n%v\n%v", chunks1, chunks2)
	}
	if !reflect.DeepEqual(postings1, postings2) {
		t.Fatalf("postings differ between runs:\n%v\n%v", postings1, postings2)
	}
}
