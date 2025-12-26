package tokenize

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/memkit/repodex/internal/config"
)

// Tokenizer provides deterministic tokenization for code, identifiers, and paths.
// It normalizes tokens, enforces length limits, removes stop words, and guarantees
// uniqueness + sorted order for determinism.
type Tokenizer struct {
	cfg        config.TokenizationConfig
	stopWords  map[string]struct{}
	allowShort map[string]struct{}
}

// StringScanState carries string-literal scanning state across calls.
type StringScanState struct {
	InString bool
	Delim    rune
	Escaped  bool
}

// New returns a Tokenizer configured from the provided tokenization config.
func New(cfg config.TokenizationConfig) Tokenizer {
	stopWords := make(map[string]struct{}, len(cfg.StopWords))
	for _, w := range cfg.StopWords {
		stopWords[strings.ToLower(w)] = struct{}{}
	}
	allowShort := make(map[string]struct{}, len(cfg.AllowShortTokens))
	for _, w := range cfg.AllowShortTokens {
		allowShort[strings.ToLower(w)] = struct{}{}
	}
	return Tokenizer{cfg: cfg, stopWords: stopWords, allowShort: allowShort}
}

// WithPath tokenizes the provided chunk text and path, returning a unique,
// sorted set of tokens.
func (t Tokenizer) WithPath(path string, chunkText string) []string {
	textTokens := t.Text(chunkText)
	pathTokens := t.Path(path)
	return mergeUnique(textTokens, pathTokens)
}

// Text tokenizes the provided text according to the tokenizer configuration.
func (t Tokenizer) Text(text string) []string {
	raw := t.scan(text, t.cfg.TokenizeStringLiterals)
	return t.normalize(raw)
}

// TextWithState tokenizes text while preserving string-literal state across calls
// when TokenizeStringLiterals is disabled. For the enabled case, it falls back to
// Text for the fastest path.
func (t Tokenizer) TextWithState(text string, st *StringScanState) []string {
	if t.cfg.TokenizeStringLiterals {
		return t.Text(text)
	}
	if st == nil {
		st = &StringScanState{}
	}
	var tokens []string
	var buf []rune
	flush := func() {
		if len(buf) > 0 {
			tokens = append(tokens, string(buf))
			buf = buf[:0]
		}
	}

	for _, r := range text {
		if st.InString {
			if st.Escaped {
				st.Escaped = false
				continue
			}
			if r == '\\' {
				st.Escaped = true
				continue
			}
			if r == st.Delim {
				st.InString = false
				continue
			}
			continue
		}
		switch r {
		case '\'', '"', '`':
			flush()
			st.InString = true
			st.Delim = r
			st.Escaped = false
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf = append(buf, r)
			continue
		}
		flush()
	}
	flush()
	return t.normalize(expandTokens(tokens))
}

// Path tokenizes a path, handling separators and extensions using the same
// normalization rules as Text.
func (t Tokenizer) Path(path string) []string {
	clean := filepath.ToSlash(path)
	base := filepath.Base(clean)
	ext := filepath.Ext(base)
	lowerBase := strings.ToLower(base)
	for _, suffix := range []string{".d.ts.map", ".d.tsx", ".d.ts"} {
		if strings.HasSuffix(lowerBase, suffix) && len(base) > len(suffix) {
			trimmedBase := base[:len(base)-len(suffix)]
			clean = strings.TrimSuffix(clean, base) + trimmedBase
			base = trimmedBase
			ext = filepath.Ext(base)
			lowerBase = strings.ToLower(base)
			break
		}
	}
	if ext != "" && len(base) > len(ext) {
		clean = strings.TrimSuffix(clean, ext)
	}
	raw := t.scan(clean, true)
	return t.normalize(raw)
}

func (t Tokenizer) scan(text string, includeStrings bool) []string {
	var tokens []string
	var buf []rune
	flush := func() {
		if len(buf) > 0 {
			tokens = append(tokens, string(buf))
			buf = buf[:0]
		}
	}

	if !includeStrings {
		var delim rune
		inString := false
		escaped := false
		for _, r := range text {
			if escaped {
				escaped = false
				continue
			}
			if inString {
				if r == '\\' {
					escaped = true
					continue
				}
				if r == delim {
					inString = false
				}
				continue
			}
			switch r {
			case '\'', '"', '`':
				flush()
				inString = true
				delim = r
				continue
			}
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				buf = append(buf, r)
				continue
			}
			flush()
		}
		flush()
		return expandTokens(tokens)
	}

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf = append(buf, r)
			continue
		}
		flush()
	}
	flush()
	return expandTokens(tokens)
}

func expandTokens(tokens []string) []string {
	var expanded []string
	for _, tok := range tokens {
		parts := splitIdentifier(tok)
		expanded = append(expanded, parts...)
	}
	return expanded
}

func splitIdentifier(tok string) []string {
	runes := []rune(tok)
	if len(runes) == 0 {
		return nil
	}
	var parts []string
	start := 0
	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		curr := runes[i]
		nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])

		switch {
		case unicode.IsDigit(prev) != unicode.IsDigit(curr):
			parts = append(parts, string(runes[start:i]))
			start = i
		case unicode.IsLower(prev) && unicode.IsUpper(curr):
			parts = append(parts, string(runes[start:i]))
			start = i
		case unicode.IsUpper(prev) && unicode.IsUpper(curr) && nextLower:
			parts = append(parts, string(runes[start:i]))
			start = i
		}
	}
	if start < len(runes) {
		parts = append(parts, string(runes[start:]))
	}
	return parts
}

func (t Tokenizer) normalize(tokens []string) []string {
	unique := make(map[string]struct{}, len(tokens))
	for _, tok := range tokens {
		lower := strings.ToLower(tok)
		if lower == "" {
			continue
		}
		if _, ok := t.stopWords[lower]; ok {
			continue
		}
		length := utf8.RuneCountInString(lower)
		if length > t.cfg.MaxTokenLen {
			continue
		}
		if length < t.cfg.MinTokenLen {
			if _, ok := t.allowShort[lower]; !ok {
				continue
			}
		}
		if isNumeric(lower) {
			continue
		}
		if t.cfg.DropHexLen > 0 && length >= t.cfg.DropHexLen && isHex(lower) {
			continue
		}
		unique[lower] = struct{}{}
	}
	out := make([]string, 0, len(unique))
	for tok := range unique {
		out = append(out, tok)
	}
	sort.Strings(out)
	return out
}

func mergeUnique(groups ...[]string) []string {
	total := 0
	for _, g := range groups {
		total += len(g)
	}
	set := make(map[string]struct{}, total)
	for _, g := range groups {
		for _, tok := range g {
			set[tok] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for tok := range set {
		out = append(out, tok)
	}
	sort.Strings(out)
	return out
}

func isNumeric(tok string) bool {
	if tok == "" {
		return false
	}
	for _, r := range tok {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isHex(tok string) bool {
	if tok == "" {
		return false
	}
	for _, r := range tok {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
