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
	MatchLine uint32   `json:"match_line,omitempty"`
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

	tok := tokenize.New(cfg.Token)
	tokens := tok.Text(q)
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

	if root != "" {
		rerankTop(root, tok, results, uniqueTerms, maxPerFile, topK)
	}

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
		enrichSnippets(root, tok, filtered, uniqueTerms, cfg.Limits.MaxSnippetBytes)
	}

	return filtered, nil
}

func enrichSnippets(root string, tok tokenize.Tokenizer, results []Result, terms []string, maxBytes int) {
	if len(results) == 0 {
		return
	}
	lineCache := make(map[string][]string)
	for i := range results {
		r := &results[i]
		if len(terms) == 0 {
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
		if r.MatchLine == 0 {
			r.MatchLine = findBestMatchLine(lines, int(r.StartLine), int(r.EndLine), terms, tok)
		}
		snippet := extractTermSnippet(lines, int(r.StartLine), int(r.EndLine), terms, maxBytes, tok)
		if snippet != "" {
			r.Snippet = snippet
		}
	}
}

func findBestMatchLine(lines []string, start, end int, terms []string, tok tokenize.Tokenizer) uint32 {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return 0
	}
	termSet := make(map[string]struct{}, len(terms))
	for _, t := range terms {
		trimmed := strings.TrimSpace(t)
		if trimmed == "" {
			continue
		}
		termSet[trimmed] = struct{}{}
	}
	if len(termSet) == 0 {
		return 0
	}

	bestLine := 0
	bestCoverage := 0
	for i := start - 1; i < end; i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		ltoks := tok.Text(line)
		if len(ltoks) == 0 {
			continue
		}
		lineSeen := make(map[string]struct{}, len(termSet))
		lineCov := 0
		for _, w := range ltoks {
			if _, ok := termSet[w]; !ok {
				continue
			}
			if _, ok := lineSeen[w]; ok {
				continue
			}
			lineSeen[w] = struct{}{}
			lineCov++
		}
		if lineCov > bestCoverage {
			bestCoverage = lineCov
			bestLine = i + 1
		}
	}
	return uint32(bestLine)
}

func extractTermSnippet(lines []string, start, end int, terms []string, maxBytes int, tok tokenize.Tokenizer) string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return ""
	}

	termSet := make(map[string]struct{}, len(terms))
	for _, t := range terms {
		trimmed := strings.TrimSpace(t)
		if trimmed == "" {
			continue
		}
		termSet[trimmed] = struct{}{}
	}
	if len(termSet) == 0 {
		return ""
	}

	type cand struct {
		idx int
		cov int
	}
	cands := make([]cand, 0, 8)
	bestCov := 0
	bestIdx := -1

	for i := start - 1; i < end; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		ltoks := tok.Text(line)
		if len(ltoks) == 0 {
			continue
		}
		lineSeen := make(map[string]struct{}, len(termSet))
		cov := 0
		for _, w := range ltoks {
			if _, ok := termSet[w]; !ok {
				continue
			}
			if _, ok := lineSeen[w]; ok {
				continue
			}
			lineSeen[w] = struct{}{}
			cov++
		}
		if cov == 0 {
			continue
		}
		cands = append(cands, cand{idx: i, cov: cov})
		if cov > bestCov || (cov == bestCov && (bestIdx == -1 || i < bestIdx)) {
			bestCov = cov
			bestIdx = i
		}
	}
	if len(cands) == 0 {
		return ""
	}

	selected := make([]int, 0, 3)
	if bestCov == len(termSet) && bestIdx >= 0 {
		selected = append(selected, bestIdx)
	} else {
		sort.Slice(cands, func(i, j int) bool {
			if cands[i].cov != cands[j].cov {
				return cands[i].cov > cands[j].cov
			}
			return cands[i].idx < cands[j].idx
		})
		seen := make(map[int]struct{}, 3)
		for _, c := range cands {
			if len(selected) >= 3 {
				break
			}
			if _, ok := seen[c.idx]; ok {
				continue
			}
			seen[c.idx] = struct{}{}
			selected = append(selected, c.idx)
		}
	}

	var b strings.Builder
	for k, idx := range selected {
		if k > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(lines[idx])
	}

	snippet := b.String()
	if maxBytes > 0 {
		raw := []byte(snippet)
		if len(raw) > maxBytes {
			raw = raw[:maxBytes]
			for len(raw) > 0 && !utf8.Valid(raw) {
				raw = raw[:len(raw)-1]
			}
			snippet = strings.TrimRight(string(raw), "\n")
		}
	}
	return snippet
}

type rerankInfo struct {
	bestLineCoverage int
	chunkCoverage    int
	spanLines        int
	isTest           bool
}

func rerankTop(root string, tok tokenize.Tokenizer, results []Result, terms []string, maxPerFile, topK int) {
	if len(results) < 2 || len(terms) == 0 {
		return
	}
	termSet := make(map[string]struct{}, len(terms))
	for _, t := range terms {
		trimmed := strings.TrimSpace(t)
		if trimmed == "" {
			continue
		}
		termSet[trimmed] = struct{}{}
	}
	if len(termSet) == 0 {
		return
	}

	limit := topK * maxPerFile * 50
	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}

	lineCache := make(map[string][]string)
	infoByID := make(map[uint32]rerankInfo, limit)

	for i := 0; i < limit; i++ {
		r := results[i]
		info := rerankInfo{
			spanLines: int(r.EndLine - r.StartLine + 1),
			isTest:    strings.HasSuffix(r.Path, "_test.go"),
		}
		lines, ok := lineCache[r.Path]
		if !ok {
			abs := filepath.Join(root, filepath.FromSlash(r.Path))
			data, err := os.ReadFile(abs)
			if err != nil {
				lineCache[r.Path] = nil
				infoByID[r.ChunkID] = info
				continue
			}
			normalized := textutil.NormalizeNewlinesBytes(data)
			lines = strings.Split(string(normalized), "\n")
			lineCache[r.Path] = lines
		}
		if lines != nil {
			bestLine, chunkCov := computeChunkMatchStats(lines, int(r.StartLine), int(r.EndLine), termSet, tok)
			info.bestLineCoverage = bestLine
			info.chunkCoverage = chunkCov
		}
		infoByID[r.ChunkID] = info
	}

	sort.SliceStable(results[:limit], func(i, j int) bool {
		ri := infoByID[results[i].ChunkID]
		rj := infoByID[results[j].ChunkID]
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if ri.bestLineCoverage != rj.bestLineCoverage {
			return ri.bestLineCoverage > rj.bestLineCoverage
		}
		if ri.chunkCoverage != rj.chunkCoverage {
			return ri.chunkCoverage > rj.chunkCoverage
		}
		if ri.isTest != rj.isTest {
			return !ri.isTest
		}
		if ri.spanLines != rj.spanLines {
			return ri.spanLines < rj.spanLines
		}
		return results[i].ChunkID < results[j].ChunkID
	})
}

func computeChunkMatchStats(lines []string, start, end int, termSet map[string]struct{}, tok tokenize.Tokenizer) (int, int) {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end || len(termSet) == 0 {
		return 0, 0
	}

	bestLine := 0
	chunkSeen := make(map[string]struct{}, len(termSet))

	for i := start - 1; i < end; i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		ltoks := tok.Text(line)
		if len(ltoks) == 0 {
			continue
		}
		lineSeen := make(map[string]struct{}, len(termSet))
		lineCov := 0
		for _, w := range ltoks {
			if _, ok := termSet[w]; !ok {
				continue
			}
			if _, ok := lineSeen[w]; ok {
				continue
			}
			lineSeen[w] = struct{}{}
			lineCov++
			if _, ok := chunkSeen[w]; !ok {
				chunkSeen[w] = struct{}{}
			}
		}
		if lineCov > bestLine {
			bestLine = lineCov
		}
	}

	return bestLine, len(chunkSeen)
}

// RoundScores rounds result scores to two decimal places for display.
func RoundScores(results []Result) {
	for i := range results {
		results[i].Score = math.Round(results[i].Score*100) / 100
	}
}
