package mediawiki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
)

const parseActionQueryString = "/api.php?action=parse" +
	"&prop=revid|wikitext|tocdata|links|templates|images|categories|externallinks" +
	"&formatversion=2&format=json"

func FetchRecentInfo(ctx context.Context, page string, locale string) (entity.ArticleInfo, error) {
	reqUrl := WikiUrl + locale + parseActionQueryString + "&page=" + url.QueryEscape(page)
	return fetchInfoByUrl(ctx, reqUrl)
}

func FetchInfoByRevId(ctx context.Context, revID int64, locale string) (entity.ArticleInfo, error) {
	reqUrl := WikiUrl + locale + parseActionQueryString + "&oldid=" + strconv.FormatInt(revID, 10)
	return fetchInfoByUrl(ctx, reqUrl)
}

func fetchInfoByUrl(ctx context.Context, reqUrl string) (entity.ArticleInfo, error) {
	var op = fmt.Sprintf("fetchInfoByUrl '%s", reqUrl)

	body, err := fetch(ctx, reqUrl)
	if err != nil {
		return entity.ArticleInfo{}, fmt.Errorf("%s: %w", op, err)
	}

	info, err := ParseInfo(body)
	if err != nil {
		return entity.ArticleInfo{}, fmt.Errorf("%s: %w", op, err)
	}

	return info, nil
}

var (
	tableRe       = regexp.MustCompile(`(?s)\{\|.*?\n\|\}`)
	htmlTagRe     = regexp.MustCompile(`<[^>]+>`)
	wikilinkRe    = regexp.MustCompile(`\[\[([^\]]*)\]\]`)
	extLinkTextRe = regexp.MustCompile(`\[https?://\S+\s+([^\]]*)\]`)
	extLinkBareRe = regexp.MustCompile(`\[https?://\S+\]`)
	headingRe     = regexp.MustCompile(`(?m)^\s*={1,6}\s*(.*?)\s*={1,6}\s*$`)
	quoteMarksRe  = regexp.MustCompile(`'{2,}`)

	fileLinkPrefixes = map[string]struct{}{
		"файл": {}, "изображение": {}, "file": {}, "image": {}, "arquivo": {},
	}
)

// ExtractWords turns wikitext into the words an editor actually wrote: markup
// (templates, tables, tags, link syntax) is stripped so it isn't counted as
// content. Table cell text is dropped along with the table skeleton - tables
// are scored by their own metric, not folded into prose length.
func ExtractWords(wikitext string) ([]string, error) {
	text := stripBalancedTemplates(wikitext)
	text = tableRe.ReplaceAllString(text, " ")
	text = htmlTagRe.ReplaceAllString(text, " ")
	text = wikilinkRe.ReplaceAllStringFunc(text, wikilinkDisplayText)
	text = extLinkTextRe.ReplaceAllString(text, " $1 ")
	text = extLinkBareRe.ReplaceAllString(text, " ")
	text = headingRe.ReplaceAllString(text, " $1 ")
	text = quoteMarksRe.ReplaceAllString(text, "")

	return strings.Fields(text), nil
}

// stripBalancedTemplates removes {{ ... }} template calls, including ones
// nested inside each other.
func stripBalancedTemplates(text string) string {
	var b strings.Builder
	depth := 0

	for i := 0; i < len(text); i++ {
		switch {
		case text[i] == '{' && i+1 < len(text) && text[i+1] == '{':
			depth++
			i++
		case depth > 0 && text[i] == '}' && i+1 < len(text) && text[i+1] == '}':
			depth--
			i++
		case depth == 0:
			b.WriteByte(text[i])
		}
	}

	return b.String()
}

// dedupTemplateBaseNames collapses subpage variants of a template into one
// entry (e.g. "Template:Rank/05" and "Template:Rank/06" both become
// "Template:Rank").
func dedupTemplateBaseNames(rawTemplates []template) []string {
	seen := make(map[string]struct{}, len(rawTemplates))
	result := make([]string, 0, len(rawTemplates))

	for _, t := range rawTemplates {
		base, _, _ := strings.Cut(t.Title, "/")
		if _, ok := seen[base]; ok {
			continue
		}

		seen[base] = struct{}{}
		result = append(result, base)
	}

	return result
}

// wikilinkDisplayText returns the visible text of a [[...]] wikilink match:
// the part after the last pipe, or the whole target if there is none. File
// and image links carry no prose, so they are dropped entirely.
func wikilinkDisplayText(match string) string {
	inner := match[2 : len(match)-2]

	if prefix, _, found := strings.Cut(inner, ":"); found {
		if _, isFile := fileLinkPrefixes[strings.ToLower(strings.TrimSpace(prefix))]; isFile {
			return " "
		}
	}

	parts := strings.Split(inner, "|")
	return " " + parts[len(parts)-1] + " "
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
		Wikitext   string     `json:"wikitext"`
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

// ParseInfo parses a raw MediaWiki action=parse API response into an ArticleInfo.
func ParseInfo(jsonResponse []byte) (entity.ArticleInfo, error) {
	var parseResp parseResponse

	if err := json.Unmarshal(jsonResponse, &parseResp); err != nil {
		return entity.ArticleInfo{}, err
	}

	if parseResp.Error.Code != "" {
		if parseResp.Error.Code == "missingtitle" {
			return entity.ArticleInfo{}, ErrPageNotFound
		}
		return entity.ArticleInfo{}, fmt.Errorf("API error: [%s] %s", parseResp.Error.Code, parseResp.Error.Info)
	}

	resp := parseResp.Parse

	words, err := ExtractWords(resp.Wikitext)
	if err != nil {
		return entity.ArticleInfo{}, err
	}

	categories := make([]string, len(resp.Categories))
	for i, cat := range resp.Categories {
		categories[i] = cat.Category
	}

	links := make([]string, len(resp.Links))
	for i, link := range resp.Links {
		links[i] = link.Title
	}

	templates := dedupTemplateBaseNames(resp.Templates)

	sections := make([]entity.Section, len(resp.TocData.Sections))
	for i, section := range resp.TocData.Sections {
		sections[i].Level = section.Level
		sections[i].Line = section.Line
	}

	return entity.ArticleInfo{
		RevId:      resp.RevId,
		Title:      resp.Title,
		Words:      words,
		Categories: categories,
		Links:      links,
		Images:     resp.Images,
		Sections:   sections,
		Templates:  templates,
		Wikitext:   resp.Wikitext,
	}, nil
}
