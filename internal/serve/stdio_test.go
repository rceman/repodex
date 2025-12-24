package serve

// This integration test drives ServeStdio through real pipes. It mutates os.Stdin and os.Stdout,
// so it must never use t.Parallel or any parallel helpers.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/fetch"
	"github.com/memkit/repodex/internal/hash"
	"github.com/memkit/repodex/internal/ignore"
	"github.com/memkit/repodex/internal/index"
	"github.com/memkit/repodex/internal/lang/factory"
	"github.com/memkit/repodex/internal/scan"
	"github.com/memkit/repodex/internal/search"
	"github.com/memkit/repodex/internal/store"
)

var ioMu sync.Mutex

type responseLine struct {
	resp Response
	raw  string
	err  error
}

func TestServeStdioIntegration(t *testing.T) {
	ioMu.Lock()
	t.Cleanup(ioMu.Unlock)

	root := t.TempDir()
	buildTestIndex(t, root)

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	t.Cleanup(func() {
		_ = stdinR.Close()
		_ = stdinW.Close()
		_ = stdoutR.Close()
		_ = stdoutW.Close()
	})

	origStdin, origStdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = stdinR, stdoutW
	t.Cleanup(func() {
		os.Stdin, os.Stdout = origStdin, origStdout
	})

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- ServeStdio(root, func() (interface{}, error) {
			return map[string]string{"status": "ok"}, nil
		}, func() (interface{}, error) {
			return map[string]string{"sync": "ok"}, nil
		})
	}()

	respCh := make(chan responseLine, 8)
	go func() {
		scanner := bufio.NewScanner(stdoutR)
		scanner.Buffer(make([]byte, 0, 1024), 8*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			var resp Response
			if err := json.Unmarshal(line, &resp); err != nil {
				respCh <- responseLine{err: fmt.Errorf("decode response %q: %w", string(line), err)}
				continue
			}
			respCh <- responseLine{resp: resp, raw: string(line)}
		}
		if err := scanner.Err(); err != nil {
			respCh <- responseLine{err: err}
		}
		close(respCh)
	}()

	writeRequest(t, stdinW, "{ invalid json\n")
	invalidResp := readResponse(t, respCh)
	if invalidResp.resp.OK {
		t.Fatalf("invalid JSON should not be OK: %s", invalidResp.raw)
	}
	if invalidResp.resp.Op != "" {
		t.Fatalf("invalid JSON should return empty op: %s", invalidResp.raw)
	}
	// Error text may include parser details; only the prefix is considered stable.
	if !strings.Contains(invalidResp.resp.Error, "invalid request") {
		t.Fatalf("expected invalid request error, got: %s", invalidResp.resp.Error)
	}

	writeRequest(t, stdinW, `{"op":"bogus"}`+"\n")
	unknownResp := readResponse(t, respCh)
	if unknownResp.resp.OK {
		t.Fatalf("unknown op should not be OK: %s", unknownResp.raw)
	}
	// The protocol fixes the unknown op response format.
	if unknownResp.resp.Op != "" || unknownResp.resp.Error != "unknown op" {
		t.Fatalf("unexpected unknown op response: %s", unknownResp.raw)
	}

	oversize := strings.Repeat("x", MaxRequestBytes+1) + "\n"
	writeRequest(t, stdinW, oversize)
	oversizeResp := readResponse(t, respCh)
	// The request size rejection string is a stable contract.
	if oversizeResp.resp.OK || oversizeResp.resp.Op != "" || oversizeResp.resp.Error != "request too large" {
		t.Fatalf("unexpected oversize response: %s", oversizeResp.raw)
	}

	searchPayload := fmt.Sprintf(`{"op":"search","q":"%s","top_k":5}`, "alpha") + "\n"
	writeRequest(t, stdinW, searchPayload)
	searchResp := readResponse(t, respCh)
	if !searchResp.resp.OK || searchResp.resp.Op != "search" {
		t.Fatalf("unexpected search response: %s", searchResp.raw)
	}
	searchResults := parseSearchResults(t, searchResp.resp.Data)
	if len(searchResults) == 0 {
		t.Fatalf("expected search results")
	}
	first := searchResults[0]
	if first.ChunkID == 0 || first.Path == "" {
		t.Fatalf("incomplete search result: %+v", first)
	}

	fetchPayload := fmt.Sprintf(`{"op":"fetch","ids":[%d],"max_lines":120}`, first.ChunkID) + "\n"
	writeRequest(t, stdinW, fetchPayload)
	fetchResp := readResponse(t, respCh)
	if !fetchResp.resp.OK || fetchResp.resp.Op != "fetch" {
		t.Fatalf("unexpected fetch response: %s", fetchResp.raw)
	}
	fetchResults := parseFetchResults(t, fetchResp.resp.Data)
	if len(fetchResults) != 1 {
		t.Fatalf("expected single fetch result, got %d", len(fetchResults))
	}
	fetched := fetchResults[0]
	if fetched.ChunkID != first.ChunkID {
		t.Fatalf("fetch chunk id mismatch, got %d want %d", fetched.ChunkID, first.ChunkID)
	}
	if len(fetched.Lines) == 0 {
		t.Fatalf("expected non-empty fetched lines")
	}
	for _, line := range fetched.Lines {
		parts := strings.SplitN(line, "| ", 2)
		if len(parts) != 2 || parts[0] == "" {
			t.Fatalf("unexpected line format: %q", line)
		}
		for _, r := range parts[0] {
			if r < '0' || r > '9' {
				t.Fatalf("line number prefix invalid: %q", line)
			}
		}
	}

	if err := stdinW.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}
	if err := <-serverErrCh; err != nil {
		t.Fatalf("server error: %v", err)
	}
	if err := stdoutW.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	for extra := range respCh {
		if extra.err != nil {
			t.Fatalf("response stream error: %v", extra.err)
		}
		t.Fatalf("unexpected extra response: %s", extra.raw)
	}
}

func TestReadLineBufferFullWithinLimit(t *testing.T) {
	content := strings.Repeat("x", 64*1024)
	reader := bufio.NewReaderSize(strings.NewReader(content+"\nnext\n"), 16)

	line, tooLarge, err := readLine(reader)
	if err != nil {
		t.Fatalf("readLine error: %v", err)
	}
	if tooLarge {
		t.Fatalf("unexpected tooLarge for buffered line")
	}
	if string(line) != content {
		t.Fatalf("line mismatch: got %d bytes", len(line))
	}

	next, tooLarge, err := readLine(reader)
	if err != nil {
		t.Fatalf("readLine second error: %v", err)
	}
	if tooLarge {
		t.Fatalf("second line unexpectedly too large")
	}
	if string(next) != "next" {
		t.Fatalf("unexpected second line: %q", next)
	}
}

func TestReadLineBufferFullTooLarge(t *testing.T) {
	content := strings.Repeat("y", MaxRequestBytes+128)
	reader := bufio.NewReaderSize(strings.NewReader(content+"\nokay\n"), 32)

	line, tooLarge, err := readLine(reader)
	if err != nil {
		t.Fatalf("readLine error: %v", err)
	}
	if !tooLarge {
		t.Fatalf("expected tooLarge flag for oversized line")
	}
	if len(line) != MaxRequestBytes {
		t.Fatalf("expected truncated line length %d, got %d", MaxRequestBytes, len(line))
	}

	next, tooLarge, err := readLine(reader)
	if err != nil {
		t.Fatalf("readLine second error: %v", err)
	}
	if tooLarge {
		t.Fatalf("second line unexpectedly too large")
	}
	if string(next) != "okay" {
		t.Fatalf("unexpected second line: %q", next)
	}
}

func writeRequest(t *testing.T, w io.Writer, payload string) {
	t.Helper()
	if _, err := io.WriteString(w, payload); err != nil {
		t.Fatalf("write request: %v", err)
	}
}

func readResponse(t *testing.T, ch <-chan responseLine) responseLine {
	t.Helper()
	resp, ok := <-ch
	if !ok {
		t.Fatalf("response channel closed unexpectedly")
	}
	if resp.err != nil {
		t.Fatalf("response error: %v", resp.err)
	}
	return resp
}

func parseSearchResults(t *testing.T, data interface{}) []search.Result {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal search data: %v", err)
	}
	var results []search.Result
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("unmarshal search data: %v", err)
	}
	return results
}

func parseFetchResults(t *testing.T, data interface{}) []fetch.ChunkText {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal fetch data: %v", err)
	}
	var results []fetch.ChunkText
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("unmarshal fetch data: %v", err)
	}
	return results
}

func TestServeStdioValidationSearch(t *testing.T) {
	resp := runValidationRequest(t, `{"op":"search","q":""}`+"\n")
	if resp.resp.OK {
		t.Fatalf("empty search should not be OK: %s", resp.raw)
	}
	if resp.resp.Op != "search" {
		t.Fatalf("empty search should keep op, got %q", resp.resp.Op)
	}
	if resp.resp.Error != "invalid search request: q is required" {
		t.Fatalf("unexpected error: %s", resp.resp.Error)
	}
}

func TestServeStdioValidationFetch(t *testing.T) {
	t.Run("missing ids", func(t *testing.T) {
		resp := runValidationRequest(t, `{"op":"fetch"}`+"\n")
		if resp.resp.OK {
			t.Fatalf("missing ids should not be OK: %s", resp.raw)
		}
		if resp.resp.Op != "fetch" {
			t.Fatalf("missing ids should keep op, got %q", resp.resp.Op)
		}
		if resp.resp.Error != "invalid fetch request: ids are required" {
			t.Fatalf("unexpected error: %s", resp.resp.Error)
		}
	})
	t.Run("too many ids", func(t *testing.T) {
		resp := runValidationRequest(t, `{"op":"fetch","ids":[1,2,3,4,5,6]}`+"\n")
		if resp.resp.OK {
			t.Fatalf("too many ids should not be OK: %s", resp.raw)
		}
		if resp.resp.Op != "fetch" {
			t.Fatalf("too many ids should keep op, got %q", resp.resp.Op)
		}
		if resp.resp.Error != "invalid fetch request: maximum 5 ids allowed" {
			t.Fatalf("unexpected error: %s", resp.resp.Error)
		}
	})
}

func runValidationRequest(t *testing.T, payload string) responseLine {
	t.Helper()
	ioMu.Lock()
	t.Cleanup(ioMu.Unlock)

	root := t.TempDir()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	t.Cleanup(func() {
		_ = stdinR.Close()
		_ = stdinW.Close()
		_ = stdoutR.Close()
		_ = stdoutW.Close()
	})

	origStdin, origStdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = stdinR, stdoutW
	t.Cleanup(func() {
		os.Stdin, os.Stdout = origStdin, origStdout
	})

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- ServeStdio(root, func() (interface{}, error) {
			return nil, nil
		}, func() (interface{}, error) {
			return nil, nil
		})
	}()

	respCh := make(chan responseLine, 1)
	go func() {
		scanner := bufio.NewScanner(stdoutR)
		scanner.Buffer(make([]byte, 0, 1024), 8*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			var resp Response
			if err := json.Unmarshal(line, &resp); err != nil {
				respCh <- responseLine{err: fmt.Errorf("decode response %q: %w", string(line), err)}
				continue
			}
			respCh <- responseLine{resp: resp, raw: string(line)}
		}
		if err := scanner.Err(); err != nil {
			respCh <- responseLine{err: err}
		}
		close(respCh)
	}()

	writeRequest(t, stdinW, payload)
	if err := stdinW.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}
	resp := readResponse(t, respCh)
	if err := <-serverErrCh; err != nil {
		t.Fatalf("server error: %v", err)
	}
	if err := stdoutW.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	for extra := range respCh {
		if extra.err != nil {
			t.Fatalf("response stream error: %v", extra.err)
		}
		t.Fatalf("unexpected extra response: %s", extra.raw)
	}
	return resp
}

func buildTestIndex(t *testing.T, root string) {
	t.Helper()
	srcDir := filepath.Join(root, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("create src dir: %v", err)
	}
	if err := os.MkdirAll(store.Dir(root), 0o755); err != nil {
		t.Fatalf("create store dir: %v", err)
	}
	sample := strings.Join([]string{
		"import { readFileSync } from 'fs'",
		"",
		"export interface Example {",
		"  id: string;",
		"  alphaBeta: number;",
		"}",
		"",
		"export function alphaBetaValue(input: number) {",
		"  const alphaBeta = input + 1;",
		"  return alphaBeta * 2;",
		"}",
		"",
		"const totals = [1, 2, 3];",
		"const doubled = totals.map((value) => alphaBetaValue(value));",
		"",
		"export const alphaHelper = (value: number) => {",
		"  return value + alphaBetaValue(value);",
		"};",
		"",
		"export const betaHelper = (value: number) => {",
		"  return alphaHelper(value) - 1;",
		"};",
		"",
		"export const summary = doubled.reduce((sum, value) => sum + value, 0);",
	}, "\n") + "\n"
	filePath := filepath.Join(srcDir, "sample.ts")
	if err := os.WriteFile(filePath, []byte(sample), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.ProjectType = "ts"
	if err := config.Save(store.ConfigPath(root), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if err := ignore.WriteDefaultIgnore(store.IgnorePath(root)); err != nil {
		t.Fatalf("write ignore: %v", err)
	}

	cfgBytes, err := os.ReadFile(store.ConfigPath(root))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	plugin, err := factory.FromProjectType(cfg.ProjectType)
	if err != nil {
		t.Fatalf("plugin: %v", err)
	}

	files, err := scan.Walk(root, cfg, nil)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	fileEntries, chunkEntries, postings, err := index.Build(files, plugin, cfg)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	if err := index.Serialize(root, fileEntries, chunkEntries, postings); err != nil {
		t.Fatalf("serialize index: %v", err)
	}
	meta := store.NewMeta(cfg.IndexVersion, len(fileEntries), len(chunkEntries), len(postings), hash.Sum64(cfgBytes))
	if err := store.SaveMeta(store.MetaPath(root), meta); err != nil {
		t.Fatalf("save meta: %v", err)
	}
}
