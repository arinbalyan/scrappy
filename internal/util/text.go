package util

import (
	"strings"
	"sync"

	"golang.org/x/net/html"
)

var builderPool = sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

// StripHTML removes all HTML tags from s, returning only the text content.
func StripHTML(s string) string {
	if s == "" || !strings.ContainsAny(s, "<>") {
		return html.UnescapeString(s)
	}
	tokenizer := html.NewTokenizer(strings.NewReader(s))
	
	sb := builderPool.Get().(*strings.Builder)
	sb.Reset()
	defer builderPool.Put(sb)
	
	sb.Grow(len(s))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return html.UnescapeString(sb.String())
		case html.TextToken:
			sb.Write(tokenizer.Text())
		default:
			// skip tags, comments, doctype, etc.
		}
	}
}
