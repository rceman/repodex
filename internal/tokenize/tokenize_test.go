package tokenize

import (
	"reflect"
	"testing"

	"github.com/memkit/repodex/internal/config"
)

func newTestCfg() config.TokenizationConfig {
	return config.TokenizationConfig{
		MinTokenLen:            2,
		MaxTokenLen:            64,
		DropHexLen:             16,
		AllowShortTokens:       []string{"id", "ui"},
		StopWords:              []string{"const"},
		TokenizeStringLiterals: true,
	}
}

func TestTokenizerSplitsIdentifiersAndPaths(t *testing.T) {
	cfg := newTestCfg()
	tok := New(cfg)

	text := "const HTTPRequest = getUserID(snake_case); foo-bar baz.qux/pathSegment1 pathSegment2;"
	tokens := tok.WithPath("api/user_service.ts", text)

	want := []string{"api", "bar", "baz", "case", "foo", "get", "http", "id", "path", "qux", "request", "segment", "service", "snake", "user"}
	if !reflect.DeepEqual(tokens, want) {
		t.Fatalf("unexpected tokens.\nwant: %v\n got: %v", want, tokens)
	}
}

func TestTokenizerRespectsMinMaxLengthAndUniqueness(t *testing.T) {
	cfg := newTestCfg()
	cfg.MaxTokenLen = 5
	tok := New(cfg)

	text := "aa bbbbb cccccc AA aa"
	tokens := tok.Text(text)

	want := []string{"aa", "bbbbb"}
	if !reflect.DeepEqual(tokens, want) {
		t.Fatalf("unexpected tokens.\nwant: %v\n got: %v", want, tokens)
	}
}

func TestTokenizerDropsStringLiteralsWhenDisabled(t *testing.T) {
	cfg := newTestCfg()
	cfg.TokenizeStringLiterals = false
	tok := New(cfg)

	text := `const label = "stringLiteral"; const id = 42;`
	tokens := tok.Text(text)

	for _, disallowed := range []string{"stringliteral"} {
		for _, tok := range tokens {
			if tok == disallowed {
				t.Fatalf("expected %s to be dropped when string literals disabled", disallowed)
			}
		}
	}
	if !contains(tokens, "label") || !contains(tokens, "id") {
		t.Fatalf("expected code tokens to remain, got %v", tokens)
	}
}

func TestTokenizerDropsHexAndNumericTokens(t *testing.T) {
	cfg := newTestCfg()
	tok := New(cfg)

	text := "aabbccddeeff0011 12345 id"
	tokens := tok.Text(text)

	if contains(tokens, "aabbccddeeff0011") {
		t.Fatalf("hex token should be dropped")
	}
	if contains(tokens, "12345") {
		t.Fatalf("numeric token should be dropped")
	}
	if !contains(tokens, "id") {
		t.Fatalf("expected allowed short token 'id'")
	}
}

func TestTokenizerTextWithStateNilDoesNotPanic(t *testing.T) {
	cfg := newTestCfg()
	cfg.TokenizeStringLiterals = false
	tok := New(cfg)

	tokens := tok.TextWithState("Foo bar baz", nil)
	for _, want := range []string{"foo", "bar", "baz"} {
		if !contains(tokens, want) {
			t.Fatalf("expected token %s, got %v", want, tokens)
		}
	}
}

func TestTokenizerTextReturnsSortedUnique(t *testing.T) {
	cfg := newTestCfg()
	cfg.MinTokenLen = 1
	tok := New(cfg)

	tokens := tok.Text("c b a A c")
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(tokens, want) {
		t.Fatalf("expected %v, got %v", want, tokens)
	}
}

func TestTokenizerPathStripsDTS(t *testing.T) {
	cfg := newTestCfg()
	tok := New(cfg)

	tokens := tok.Path("types/react/index.d.ts")
	for _, disallowed := range []string{"d", "ts"} {
		if contains(tokens, disallowed) {
			t.Fatalf("unexpected token %s from d.ts path", disallowed)
		}
	}
	for _, want := range []string{"types", "react", "index"} {
		if !contains(tokens, want) {
			t.Fatalf("expected token %s from path, got %v", want, tokens)
		}
	}
}

func TestTokenizerPathTrimmingMatrix(t *testing.T) {
	cfg := newTestCfg()
	tok := New(cfg)

	cases := []struct {
		path     string
		expect   []string
		disallow []string
	}{
		{"src/index.ts", []string{"src", "index"}, []string{"ts"}},
		{"src/index.tsx", []string{"src", "index"}, []string{"tsx"}},
		{"types/react/index.d.ts", []string{"types", "react", "index"}, []string{"d", "ts"}},
		{"types/react/index.d.tsx", []string{"types", "react", "index"}, []string{"d", "ts", "tsx"}},
		{"types/react/index.d.ts.map", []string{"types", "react", "index"}, []string{"d", "ts", "map"}},
		{"src/foo.test.ts", []string{"src", "foo", "test"}, []string{"ts"}},
		{"src/foo.spec.tsx", []string{"src", "foo", "spec"}, []string{"tsx"}},
	}

	for _, tc := range cases {
		tokens := tok.Path(tc.path)
		for _, want := range tc.expect {
			if !contains(tokens, want) {
				t.Fatalf("path %s expected token %s, got %v", tc.path, want, tokens)
			}
		}
		for _, bad := range tc.disallow {
			if contains(tokens, bad) {
				t.Fatalf("path %s unexpected token %s in %v", tc.path, bad, tokens)
			}
		}
	}
}

func TestTokenizerTextWithStateDeterministic(t *testing.T) {
	cfg := newTestCfg()
	cfg.TokenizeStringLiterals = false
	tok := New(cfg)

	var st StringScanState
	first := tok.TextWithState("foo foo bar", &st)
	second := tok.TextWithState("baz foo", &st)

	combined := append([]string{}, first...)
	combined = append(combined, second...)

	var st2 StringScanState
	first2 := tok.TextWithState("foo foo bar", &st2)
	second2 := tok.TextWithState("baz foo", &st2)
	combined2 := append([]string{}, first2...)
	combined2 = append(combined2, second2...)

	if !reflect.DeepEqual(combined, combined2) {
		t.Fatalf("determinism failed: %v vs %v", combined, combined2)
	}
}

func TestTokenizerSkipsEscapedQuotes(t *testing.T) {
	cfg := newTestCfg()
	cfg.TokenizeStringLiterals = false
	tok := New(cfg)

	text := "const msg = \"secret\\\"value\"; const after = code;"
	tokens := tok.Text(text)
	if contains(tokens, "secret") || contains(tokens, "value") {
		t.Fatalf("string literal content should be skipped, got %v", tokens)
	}
	for _, want := range []string{"msg", "after", "code"} {
		if !contains(tokens, want) {
			t.Fatalf("expected token %s, got %v", want, tokens)
		}
	}
}

func TestTokenizerDropsMultilineTemplateLiteralsWhenDisabled(t *testing.T) {
	cfg := newTestCfg()
	cfg.TokenizeStringLiterals = false
	tok := New(cfg)

	lines := []string{
		"const tpl = `line1 SECRET_TOKEN_ABC",
		"${value} moreSecret`",
		"const fooBar = 1;",
	}
	var st StringScanState
	set := make(map[string]struct{})
	for _, line := range lines {
		for _, tok := range tok.TextWithState(line, &st) {
			set[tok] = struct{}{}
		}
	}
	for _, disallowed := range []string{"secret", "token", "abc", "moresecret"} {
		if _, ok := set[disallowed]; ok {
			t.Fatalf("unexpected token %s from template literal", disallowed)
		}
	}
	if _, ok := set["foo"]; !ok {
		t.Fatalf("expected token foo from code")
	}
	if _, ok := set["bar"]; !ok {
		t.Fatalf("expected token bar from code")
	}
}

func TestTokenizerStateDoesNotLeakAfterClosingBacktick(t *testing.T) {
	cfg := newTestCfg()
	cfg.TokenizeStringLiterals = false
	tok := New(cfg)

	lines := []string{
		"`hiddenValue`",
		"visibleToken anotherOne",
	}
	var st StringScanState
	var tokens []string
	for _, line := range lines {
		tokens = append(tokens, tok.TextWithState(line, &st)...)
	}
	if contains(tokens, "hiddenvalue") {
		t.Fatalf("template literal content should be skipped")
	}
	for _, want := range []string{"visible", "token", "another", "one"} {
		if !contains(tokens, want) {
			t.Fatalf("expected token %s after closing backtick, got %v", want, tokens)
		}
	}
}

func TestTokenizerEscapedBacktickDoesNotCloseTemplate(t *testing.T) {
	cfg := newTestCfg()
	cfg.TokenizeStringLiterals = false
	tok := New(cfg)

	lines := []string{
		"const tpl = `hello \\` world",
		"stillInside` const afterString = 1;",
	}
	var st StringScanState
	var tokens []string
	for _, line := range lines {
		tokens = append(tokens, tok.TextWithState(line, &st)...)
	}
	for _, disallowed := range []string{"hello", "world", "stillinside"} {
		if contains(tokens, disallowed) {
			t.Fatalf("unexpected token %s from escaped template content", disallowed)
		}
	}
	for _, want := range []string{"tpl", "after", "string"} {
		if !contains(tokens, want) {
			t.Fatalf("expected token %s after template close, got %v", want, tokens)
		}
	}
}

func contains(tokens []string, want string) bool {
	for _, tok := range tokens {
		if tok == want {
			return true
		}
	}
	return false
}
