package serve

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/memkit/repodex/internal/fetch"
	"github.com/memkit/repodex/internal/search"
)

// MaxRequestBytes limits the size of a single stdio request line.
const MaxRequestBytes = 1 << 20

// Request describes a stdio request.
type Request struct {
	Op       string   `json:"op"`
	Q        string   `json:"q,omitempty"`
	TopK     int      `json:"top_k,omitempty"`
	IDs      []uint32 `json:"ids,omitempty"`
	MaxLines int      `json:"max_lines,omitempty"`
	JSON     bool     `json:"json,omitempty"`
}

// Response describes a stdio response.
type Response struct {
	OK    bool        `json:"ok"`
	Op    string      `json:"op"`
	Error string      `json:"error,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}

// ServeStdio runs the JSONL stdio server.
func ServeStdio(root string, statusFn func() (interface{}, error), syncFn func() (interface{}, error)) error {
	reader := bufio.NewReader(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	cache := &IndexCache{}

	for {
		line, tooLarge, err := readLine(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if tooLarge {
			_ = encoder.Encode(Response{OK: false, Op: "", Error: "request too large"})
			continue
		}
		if len(line) == 0 {
			_ = encoder.Encode(Response{OK: false, Op: "", Error: "invalid request: empty"})
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			_ = encoder.Encode(Response{OK: false, Op: "", Error: fmt.Sprintf("invalid request: %v", err)})
			continue
		}

		resp := Response{OK: true, Op: req.Op}
		switch req.Op {
		case "status":
			data, err := statusFn()
			if err != nil {
				resp.OK = false
				resp.Error = err.Error()
				break
			}
			resp.Data = data
		case "sync":
			data, err := syncFn()
			if err != nil {
				resp.OK = false
				resp.Error = err.Error()
				break
			}
			cache.Invalidate()
			resp.Data = data
		case "search":
			if strings.TrimSpace(req.Q) == "" {
				resp.OK = false
				resp.Op = ""
				resp.Error = "invalid search request: q is required"
				break
			}
			if err := cache.Load(root); err != nil {
				resp.OK = false
				resp.Error = err.Error()
				break
			}
			cfg, _, plugin, chunks, _, terms, postings := cache.Get()
			results, err := search.SearchWithIndex(cfg, plugin, chunks, terms, postings, req.Q, search.Options{TopK: req.TopK})
			if err != nil {
				resp.OK = false
				resp.Error = err.Error()
				break
			}
			resp.Data = results
		case "fetch":
			if len(req.IDs) == 0 {
				resp.OK = false
				resp.Op = ""
				resp.Error = "invalid fetch request: ids are required"
				break
			}
			if len(req.IDs) > 5 {
				resp.OK = false
				resp.Op = ""
				resp.Error = "invalid fetch request: maximum 5 ids allowed"
				break
			}
			if err := cache.Load(root); err != nil {
				resp.OK = false
				resp.Error = err.Error()
				break
			}
			_, _, _, _, chunkMap, _, _ := cache.Get()
			results, err := fetch.FetchWithChunkMap(root, chunkMap, req.IDs, req.MaxLines)
			if err != nil {
				resp.OK = false
				resp.Error = err.Error()
				break
			}
			resp.Data = results
		default:
			resp.OK = false
			resp.Error = "unknown op"
			resp.Op = ""
		}
		_ = encoder.Encode(resp)
	}
	return nil
}

func readLine(reader *bufio.Reader) ([]byte, bool, error) {
	var buf bytes.Buffer
	for {
		chunk, err := reader.ReadBytes('\n')
		if len(chunk) > 0 {
			if buf.Len()+len(chunk) > MaxRequestBytes {
				allowed := MaxRequestBytes - buf.Len()
				if allowed > 0 {
					buf.Write(chunk[:allowed])
				}
				if chunk[len(chunk)-1] != '\n' {
					if err := discardUntilNewline(reader); err != nil && err != io.EOF {
						return nil, true, err
					}
				}
				return bytes.TrimRight(buf.Bytes(), "\r\n"), true, nil
			}
			buf.Write(chunk)
			if chunk[len(chunk)-1] == '\n' {
				return bytes.TrimRight(buf.Bytes(), "\r\n"), false, nil
			}
		}
		if err != nil {
			if err == io.EOF {
				if buf.Len() == 0 {
					return nil, false, io.EOF
				}
				return bytes.TrimRight(buf.Bytes(), "\r\n"), false, nil
			}
			if err == bufio.ErrBufferFull {
				continue
			}
			return nil, false, err
		}
	}
}

func discardUntilNewline(reader *bufio.Reader) error {
	for {
		chunk, err := reader.ReadBytes('\n')
		if len(chunk) > 0 && chunk[len(chunk)-1] == '\n' {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
