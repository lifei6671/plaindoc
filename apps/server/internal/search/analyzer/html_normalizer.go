package analyzer

import (
	"html"
	"regexp"
	"strings"
)

var htmlTagPattern = regexp.MustCompile(`(?s)<[^>]+>`)

// NormalizeHTMLToPlainText 将 HTML 正文规整为可索引的纯文本。
func NormalizeHTMLToPlainText(source string) string {
	normalized := strings.ReplaceAll(source, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = html.UnescapeString(normalized)
	normalized = htmlTagPattern.ReplaceAllString(normalized, " ")
	return strings.Join(strings.Fields(normalized), " ")
}
