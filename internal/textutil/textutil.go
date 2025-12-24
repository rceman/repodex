package textutil

import (
	"bytes"
	"strings"
)

var newlineReplacer = strings.NewReplacer("\r\n", "\n", "\r", "\n")

// NormalizeNewlinesBytes replaces CRLF and CR newlines with LF in byte slices.
func NormalizeNewlinesBytes(content []byte) []byte {
	if len(content) == 0 {
		return content
	}
	if !bytes.Contains(content, []byte{'\r'}) {
		return content
	}
	return []byte(newlineReplacer.Replace(string(content)))
}

// NormalizeNewlinesString replaces CRLF and CR newlines with LF in strings.
func NormalizeNewlinesString(text string) string {
	if text == "" || !strings.Contains(text, "\r") {
		return text
	}
	return newlineReplacer.Replace(text)
}
