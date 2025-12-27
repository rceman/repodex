package search

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/index"
	"github.com/memkit/repodex/internal/profile"
	"github.com/memkit/repodex/internal/store"
	"github.com/memkit/repodex/internal/textutil"
	"github.com/memkit/repodex/internal/tokenize"
)

// Options controls search behavior.
type Options struct {
	TopK       int
	MaxPerFile int
}

// Result represents a ranked chunk.
type Result struct {
	ChunkID   uint32   `json:"chunk_id"`
	Path      string   `json:"path"`
	StartLine uint32   `json:"start_line"`
	EndLine   uint32   `json:"end_line"`
	Score     float64  `json:"score"`
	Snippet   string   `json:"snippet"`
	Why       []string `json:"why"`
}

// Search executes a keyword search over the serialized index.
func Search(root string, q string, opts Options) ([]Result, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 20
	}
	if topK > 20 {
		topK = 20
	}
	maxPerFile := opts.MaxPerFile
	if maxPerFile <= 0 {
		maxPerFile = 2
	}

	userCfg, _, err := config.LoadUserConfig(store.ConfigPath(root))
	if err != nil {
		return nil, err
	}
	cfg, profiles, err := config.ApplyOverrides(config.DefaultRuntimeConfig(), userCfg)
	if err != nil {
		return nil, err
	}
	rules, err := profile.BuildEffectiveRules(root, profiles, cfg)
	if err != nil {
		return nil, err
	}
	cfg = profile.ApplyRules(cfg, rules)
	chunks, err := index.LoadChunkEntries(store.ChunksPath(root))
	if err != nil {
		return nil, err
	}
	terms, _, err := index.LoadTerms(store.TermsPath(root))
	if err != nil {
		return nil, err
	}
	postings, err := index.LoadPostings(store.PostingsPath(root))
	if err != nil {
		return nil, err
	}

	return SearchWithIndex(root, cfg, chunks, nil, terms, postings, q, Options{TopK: topK, MaxPerFile: maxPerFile})
}

// SearchWithIndex executes a keyword search using provided index data.
func SearchWithIndex(root string, cfg config.Config, chunks []index.ChunkEntry, chunkMap map[uint32]index.ChunkEntry, terms map[string]index.TermInfo, postings []uint32, q string, opts Options) ([]Result, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 20
	}
	if topK > 20 {
		topK = 20
	}
	maxPerFile := opts.MaxPerFile
	if maxPerFile <= 0 {
		maxPerFile = 2
	}

	tokens := tokenize.New(cfg.Token).Text(q)
	uniqueTerms := make([]string, 0, len(tokens))
	seen := make(map[string]struct{})
	for _, tok := range tokens {
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		uniqueTerms = append(uniqueTerms, tok)
	}

	if len(uniqueTerms) == 0 || len(chunks) == 0 {
		return nil, nil
	}

	if chunkMap == nil {
		chunkMap = make(map[uint32]index.ChunkEntry, len(chunks))
		for _, ch := range chunks {
			chunkMap[ch.ChunkID] = ch
		}
	}

	N := float64(len(chunks))
	scores := make(map[uint32]float64)
	why := make(map[uint32][]string)

	for _, term := range uniqueTerms {
		info, ok := terms[term]
		if !ok {
			continue
		}
		if info.DF == 0 {
			continue
		}
		idf := math.Log(1 + N/float64(info.DF))
		start := info.Offset / 4
		end := start + uint64(info.DF)
		if end > uint64(len(postings)) {
			return nil, fmt.Errorf("postings out of range for term %s", term)
		}
		for _, chunkID := range postings[start:end] {
			scores[chunkID] += idf
			why[chunkID] = append(why[chunkID], term)
		}
	}

	results := make([]Result, 0, len(scores))
	for id, score := range scores {
		ch, ok := chunkMap[id]
		if !ok {
			return nil, fmt.Errorf("missing chunk %d", id)
		}
		results = append(results, Result{
			ChunkID:   id,
			Path:      ch.Path,
			StartLine: ch.StartLine,
			EndLine:   ch.EndLine,
			Score:     score,
			Snippet:   ch.Snippet,
			Why:       why[id],
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].ChunkID < results[j].ChunkID
		}
		return results[i].Score > results[j].Score
	})

	filtered := make([]Result, 0, len(results))
	perFileCount := make(map[string]int)
	for _, r := range results {
		if perFileCount[r.Path] >= maxPerFile {
			continue
		}
		perFileCount[r.Path]++
		filtered = append(filtered, r)
		if len(filtered) >= topK {
			break
		}
	}

	if root != "" {
		enrichSnippets(root, filtered, cfg.Limits.MaxSnippetBytes)
	}

	return filtered, nil
}

func enrichSnippets(root string, results []Result, maxBytes int) {
	if len(results) == 0 {
		return
	}
	lineCache := make(map[string][]string)
	for i := range results {
		r := &results[i]
		if len(r.Why) == 0 {
			continue
		}
		lines, ok := lineCache[r.Path]
		if !ok {
			abs := filepath.Join(root, filepath.FromSlash(r.Path))
			data, err := os.ReadFile(abs)
			if err != nil {
				lineCache[r.Path] = nil
				continue
			}
			normalized := textutil.NormalizeNewlinesBytes(data)
			lines = strings.Split(string(normalized), "\n")
			lineCache[r.Path] = lines
		}
		if lines == nil {
			continue
		}
		snippet := extractTermSnippet(lines, int(r.StartLine), int(r.EndLine), r.Why, maxBytes)
		if snippet != "" {
			r.Snippet = snippet
		}
	}
}

func extractTermSnippet(lines []string, start, end int, terms []string, maxBytes int) string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return ""
	}
	lowerTerms := make([]string, 0, len(terms))
	for _, t := range terms {
		trimmed := strings.ToLower(strings.TrimSpace(t))
		if trimmed != "" {
			lowerTerms = append(lowerTerms, trimmed)
		}
	}
	if len(lowerTerms) == 0 {
		return ""
	}

	var picked []string
	for i := start - 1; i < end && len(picked) < 3; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		for _, term := range lowerTerms {
			if strings.Contains(lower, term) {
				picked = append(picked, line)
				break
			}
		}
	}
	if len(picked) == 0 {
		return ""
	}
	snippet := strings.Join(picked, "\n")
	if maxBytes > 0 {
		b := []byte(snippet)
		if len(b) > maxBytes {
			b = b[:maxBytes]
			for len(b) > 0 && !utf8.Valid(b) {
				b = b[:len(b)-1]
			}
			snippet = string(b)
		}
	}
	return snippet
}

// RoundScores rounds result scores to two decimal places for display.
func RoundScores(results []Result) {
	for i := range results {
		results[i].Score = math.Round(results[i].Score*100) / 100
	}
}
