package edits

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/the-new-day/protanki-wiki-admin/internal/analyze"
	"golang.org/x/net/html"
)

const parseActionQueryString = "/api.php?action=parse&prop=revid|text|tocdata|links|templates|images|categories|externallinks&formatversion=2&format=json"

func FetchInfo(page string, locale string) (analyze.Info, error) {
	url := wikiUrl + locale + parseActionQueryString + "&page=" + url.QueryEscape(page)
	return fetchInfoByUrl(url)
}

func FetchInfoById(oldId int, locale string) (analyze.Info, error) {
	url := wikiUrl + locale + parseActionQueryString + "&oldid=" + strconv.Itoa(oldId)
	return fetchInfoByUrl(url)
}

func fetchInfoByUrl(url string) (analyze.Info, error) {
	body, err := fetch(url)
	if err != nil {
		return analyze.Info{}, err
	}
	return parseInfo(body)
}

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

type category struct {
	Category string `json:"category"`
}

type link struct {
	Title string `json:"title"`
}

type template struct {
	Title string `json:"title"`
}

type tocSection struct {
	Level int    `json:"hLevel"`
	Line  string `json:"line"`
}

type tocData struct {
	Sections []tocSection `json:"sections"`
}

type parseResponse struct {
	Parse struct {
		RevId      int        `json:"revid"`
		Title      string     `json:"title"`
		Text       string     `json:"text"`
		Categories []category `json:"categories"`
		Links      []link     `json:"links"`
		Templates  []template `json:"templates"`
		Images     []string   `json:"images"`
		TocData    tocData    `json:"tocdata"`
	} `json:"parse"`
	Error struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

func parseInfo(jsonResponse []byte) (analyze.Info, error) {
	var parseResp parseResponse

	if err := json.Unmarshal(jsonResponse, &parseResp); err != nil {
		return analyze.Info{}, err
	}

	if parseResp.Error.Code != "" {
		if parseResp.Error.Code == "missingtitle" {
			return analyze.Info{}, ErrPageNotFound
		}
		return analyze.Info{}, fmt.Errorf("API error: [%s] %s", parseResp.Error.Code, parseResp.Error.Info)
	}

	resp := parseResp.Parse

	words, err := ExtractWords(resp.Text)
	if err != nil {
		return analyze.Info{}, err
	}

	categories := make([]string, len(resp.Categories))
	for i, cat := range resp.Categories {
		categories[i] = cat.Category
	}

	links := make([]string, len(resp.Links))
	for i, link := range resp.Links {
		links[i] = link.Title
	}

	templates := make([]string, len(resp.Templates))
	for i, template := range resp.Templates {
		templates[i] = template.Title
	}

	sections := make([]analyze.Section, len(resp.TocData.Sections))
	for i, section := range resp.TocData.Sections {
		sections[i].Level = section.Level
		sections[i].Line = section.Line
	}

	return analyze.Info{
		RevId:      resp.RevId,
		Title:      resp.Title,
		Words:      words,
		Categories: categories,
		Links:      links,
		Images:     resp.Images,
		Sections:   sections,
		Templates:  templates,
	}, nil
}
