package articleinfo

import (
	"strings"

	"golang.org/x/net/html"
)

func ExtractWords(htmlContent string) ([]string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	extractText(doc, &buf)

	text := strings.TrimSpace(buf.String())
	return strings.Fields(text), nil
}

func extractText(n *html.Node, buf *strings.Builder) {
	if n.Type == html.TextNode {
		buf.WriteString(n.Data)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, buf)
	}
}
