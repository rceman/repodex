package ts

import (
	"strings"
	"testing"

	"github.com/memkit/repodex/internal/config"
)

func TestTokenizerSplitsCamelAndSnake(t *testing.T) {
	cfg := config.TokenizationConfig{
		MinTokenLen:            3,
		MaxTokenLen:            64,
		DropHexLen:             16,
		AllowShortTokens:       []string{"api", "id"},
		StopWords:              []string{"const"},
		TokenizeStringLiterals: true,
		MaxFileBytesCode:       1024,
	}
	text := "const camelCaseVar = snake_case_var;"
	tokens := Tokenize("file.ts", text, cfg)

	expect := []string{"camel", "case", "var", "snake", "case", "var"}
	for _, e := range expect {
		if !contains(tokens, e) {
			t.Fatalf("expected token %s", e)
		}
	}
	if contains(tokens, "const") {
		t.Fatalf("stop word should be removed")
	}
}

func TestTokenizerHexFilter(t *testing.T) {
	cfg := config.TokenizationConfig{
		MinTokenLen:            3,
		MaxTokenLen:            64,
		DropHexLen:             16,
		AllowShortTokens:       []string{"api", "id"},
		StopWords:              []string{},
		TokenizeStringLiterals: true,
		MaxFileBytesCode:       1024,
	}
	text := "const value = aabbccddeeff0011;"
	tokens := Tokenize("file.ts", text, cfg)
	if contains(tokens, "aabbccddeeff0011") {
		t.Fatalf("hex token should be filtered")
	}
}

func TestTokenizerAllowsShortTokensAndPath(t *testing.T) {
	cfg := config.TokenizationConfig{
		MinTokenLen:            3,
		MaxTokenLen:            64,
		DropHexLen:             16,
		AllowShortTokens:       []string{"API", "ID"},
		StopWords:              []string{"CONST"},
		TokenizeStringLiterals: true,
		MaxFileBytesCode:       1024,
	}
	text := "const id = apiCall();"
	tokens := Tokenize("api/user_id.ts", text, cfg)
	if contains(tokens, "const") {
		t.Fatalf("stop word should be filtered regardless of casing")
	}
	if !contains(tokens, "api") {
		t.Fatalf("expected api token")
	}
	if !contains(tokens, "id") {
		t.Fatalf("expected id token")
	}
	if !contains(tokens, "user") {
		t.Fatalf("expected path token from path")
	}
}

func TestTokenizerUnicodeCamelSplit(t *testing.T) {
	cfg := config.TokenizationConfig{
		MinTokenLen:            3,
		MaxTokenLen:            64,
		DropHexLen:             16,
		AllowShortTokens:       []string{},
		StopWords:              []string{},
		TokenizeStringLiterals: true,
		MaxFileBytesCode:       1024,
	}
	text := "const caféName = 1;"
	tokens := Tokenize("unicode.ts", text, cfg)
	if !contains(tokens, "café") || !contains(tokens, "name") {
		t.Fatalf("expected unicode-safe camel split, got %v", tokens)
	}
}

func contains(list []string, token string) bool {
	for _, t := range list {
		if strings.EqualFold(t, token) {
			return true
		}
	}
	return false
}
