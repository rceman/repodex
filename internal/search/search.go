package search

import (
	"fmt"
	"math"
	"sort"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/index"
	"github.com/memkit/repodex/internal/lang"
	"github.com/memkit/repodex/internal/lang/factory"
	"github.com/memkit/repodex/internal/store"
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

	cfg, _, err := config.Load(store.ConfigPath(root))
	if err != nil {
		return nil, err
	}
	plugin, err := factory.FromProjectType(cfg.ProjectType)
	if err != nil {
		return nil, err
	}

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

	return SearchWithIndex(cfg, plugin, chunks, nil, terms, postings, q, Options{TopK: topK, MaxPerFile: maxPerFile})
}

// SearchWithIndex executes a keyword search using provided index data.
func SearchWithIndex(cfg config.Config, plugin lang.LanguagePlugin, chunks []index.ChunkEntry, chunkMap map[uint32]index.ChunkEntry, terms map[string]index.TermInfo, postings []uint32, q string, opts Options) ([]Result, error) {
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

	tokens := plugin.TokenizeChunk("", q, cfg.Token)
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

	return filtered, nil
}
