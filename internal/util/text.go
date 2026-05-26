package util

import (
	"strings"

	"golang.org/x/net/html"
)

// StripHTML removes all HTML tags from s, returning only the text content.
func StripHTML(s string) string {
	if s == "" || !strings.ContainsAny(s, "<>") {
		return html.UnescapeString(s)
	}
	tokenizer := html.NewTokenizer(strings.NewReader(s))
	var out strings.Builder
	out.Grow(len(s))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return html.UnescapeString(out.String())
		case html.TextToken:
			out.Write(tokenizer.Text())
		default:
			// skip tags, comments, doctype, etc.
		}
	}
}
