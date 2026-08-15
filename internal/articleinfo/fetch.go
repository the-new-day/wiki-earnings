package articleinfo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const wikiUrl = "https://wiki.pro-tanki.online/"
const queryString = "/api.php?action=parse&prop=text|tocdata|links|templates|images|categories|externallinks&formatversion=2&format=json&page="

func FetchInfo(page string, locale string) (Info, error) {
	url := wikiUrl + locale + queryString + url.QueryEscape(page)

	resp, err := http.Get(url)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Info{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return Info{}, fmt.Errorf("status: %d, body: %s", resp.StatusCode, string(body))
	}

	return parseInfo(body)
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

type response struct {
	Parse struct {
		Title      string     `json:"title"`
		Text       string     `json:"text"`
		Categories []category `json:"categories"`
		Links      []link     `json:"links"`
		Templates  []template `json:"templates"`
		Images     []string   `json:"images"`
		TocData    tocData    `json:"tocdata"`
	}
}

func parseInfo(jsonResponse []byte) (Info, error) {
	var parseResp response

	if err := json.Unmarshal(jsonResponse, &parseResp); err != nil {
		return Info{}, err
	}

	resp := parseResp.Parse

	words, err := ExtractWords(resp.Text)
	if err != nil {
		return Info{}, err
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

	sections := make([]Section, len(resp.TocData.Sections))
	for i, section := range resp.TocData.Sections {
		sections[i].Level = section.Level
		sections[i].Line = section.Line
	}

	return Info{
		Title:      resp.Title,
		Words:      words,
		Categories: categories,
		Links:      links,
		Images:     resp.Images,
		Sections:   sections,
		Templates:  templates,
	}, nil
}
