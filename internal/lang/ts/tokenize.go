package ts

import (
	"path/filepath"
	"strings"
	"unicode"

	"github.com/memkit/repodex/internal/config"
)

// Tokenize extracts tokens from chunk text and file path.
func Tokenize(path string, chunkText string, cfg config.TokenizationConfig) []string {
	text := chunkText
	if !cfg.TokenizeStringLiterals {
		text = stripQuoted(text)
	}

	terms := collectTokens(text)
	pathTerms := tokenizePath(path)
	terms = append(terms, pathTerms...)

	filtered := filterTokens(terms, cfg)
	return filtered
}

func collectTokens(text string) []string {
	var tokens []string
	current := strings.Builder{}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			current.WriteRune(r)
			continue
		}
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	var expanded []string
	for _, tok := range tokens {
		expanded = append(expanded, expandToken(tok)...)
	}
	return expanded
}

func expandToken(tok string) []string {
	parts := strings.Split(tok, "_")
	var out []string
	for _, part := range parts {
		if part == "" {
			continue
		}
		camelParts := splitCamel(part)
		out = append(out, camelParts...)
	}
	return out
}

func splitCamel(tok string) []string {
	runes := []rune(tok)
	if len(runes) == 0 {
		return nil
	}
	var parts []string
	current := []rune{runes[0]}
	for i := 1; i < len(runes); i++ {
		r := runes[i]
		prev := runes[i-1]
		if unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
			parts = append(parts, string(current))
			current = []rune{r}
			continue
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}

func tokenizePath(path string) []string {
	path = filepath.ToSlash(path)
	segments := strings.Split(path, "/")
	var tokens []string
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		seg = trimExtensions(seg)
		tokens = append(tokens, expandToken(seg)...)
	}
	return tokens
}

func trimExtensions(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range []string{".tsx", ".ts"} {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}

func filterTokens(tokens []string, cfg config.TokenizationConfig) []string {
	var out []string
	stop := make(map[string]struct{}, len(cfg.StopWords))
	for _, s := range cfg.StopWords {
		stop[strings.ToLower(s)] = struct{}{}
	}
	allowShort := make(map[string]struct{}, len(cfg.AllowShortTokens))
	for _, s := range cfg.AllowShortTokens {
		allowShort[strings.ToLower(s)] = struct{}{}
	}

	for _, tok := range tokens {
		tok = strings.ToLower(tok)
		if tok == "" {
			continue
		}
		if _, ok := stop[tok]; ok {
			continue
		}
		if len(tok) < cfg.MinTokenLen {
			if _, ok := allowShort[tok]; !ok {
				continue
			}
		}
		if len(tok) > cfg.MaxTokenLen {
			continue
		}
		if isNumeric(tok) {
			continue
		}
		if len(tok) >= cfg.DropHexLen && isHex(tok) {
			continue
		}
		out = append(out, tok)
	}
	return out
}

func isNumeric(tok string) bool {
	for _, r := range tok {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isHex(tok string) bool {
	for _, r := range tok {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func stripQuoted(text string) string {
	var out []rune
	inString := false
	var delim rune
	escaped := false
	for _, r := range text {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inString {
			escaped = true
			continue
		}
		if inString {
			if r == delim {
				inString = false
			}
			continue
		}
		if r == '"' || r == '\'' || r == '`' {
			inString = true
			delim = r
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
