package serve

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/memkit/repodex/internal/fetch"
	"github.com/memkit/repodex/internal/search"
)

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
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			_ = encoder.Encode(Response{OK: false, Error: fmt.Sprintf("invalid request: %v", err)})
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
			resp.Data = data
		case "search":
			results, err := search.Search(root, req.Q, search.Options{TopK: req.TopK})
			if err != nil {
				resp.OK = false
				resp.Error = err.Error()
				break
			}
			resp.Data = results
		case "fetch":
			results, err := fetch.Fetch(root, req.IDs, req.MaxLines)
			if err != nil {
				resp.OK = false
				resp.Error = err.Error()
				break
			}
			resp.Data = results
		default:
			resp.OK = false
			resp.Error = "unknown op"
		}
		_ = encoder.Encode(resp)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
