package mediawiki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
)

const parseActionQueryString = "/api.php?action=parse&prop=revid|wikitext|tocdata|links|templates|images|categories|externallinks&formatversion=2&format=json"

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

	info, err := parseInfo(body)
	if err != nil {
		return entity.ArticleInfo{}, fmt.Errorf("%s: %w", op, err)
	}

	return info, nil
}

func ExtractWords(wikitext string) ([]string, error) {
	return strings.Fields(wikitext), nil // TODO: parse templates and other stuff
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

func parseInfo(jsonResponse []byte) (entity.ArticleInfo, error) {
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

	templates := make([]string, len(resp.Templates))
	for i, template := range resp.Templates {
		templates[i] = template.Title
	}

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
	}, nil
}
