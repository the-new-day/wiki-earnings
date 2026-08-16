package parse

import (
	"encoding/json"
	"errors"
	"net/url"

	"github.com/the-new-day/protanki-wiki-admin/internal/analyze"
	"github.com/the-new-day/protanki-wiki-admin/internal/utils"
)

var (
	ErrPageNotFound = errors.New("page not found")
)

const (
	MinorEditTag             = "(ME)"
	ItemAdditionEditTag      = "(IA)"
	ArticleEditTag           = "(AE)"
	RefactoredArticleEditTag = "(RA)"
	TranslatedArticleEditTag = "(TA)"
	NewArticleEditTag        = "(NA)"
)

func possibleTags() []string {
	return []string{
		MinorEditTag, ItemAdditionEditTag, ArticleEditTag,
		RefactoredArticleEditTag, TranslatedArticleEditTag, NewArticleEditTag,
	}
}

const getHistoryQueryString = "/api.php?action=query&prop=revisions&rvlimit=500&rvprop=ids|comment&format=json&formatversion=2"

func FetchLastEdit(page string, locale string) (analyze.Edit, error) {
	url := wikiUrl + locale + getHistoryQueryString + "&titles=" + url.QueryEscape(page)

	body, err := fetch(url)
	if err != nil {
		return analyze.Edit{}, err
	}

	return parseLastEdit(body, locale)
}

type rev struct {
	RevId   int    `json:"revid"`
	Comment string `json:"comment"`
}

type page struct {
	PageId    int    `json:"pageid"`
	Title     string `json:"title"`
	Revisions []rev  `json:"revisions"`
}

type historyResponse struct {
	Query struct {
		Pages []page `json:"pages"`
	} `json:"query"`
}

func parseLastEdit(jsonResponse []byte, locale string) (analyze.Edit, error) {
	var historyResp historyResponse

	if err := json.Unmarshal(jsonResponse, &historyResp); err != nil {
		return analyze.Edit{}, err
	}

	if len(historyResp.Query.Pages) == 0 {
		return analyze.Edit{}, ErrPageNotFound
	}

	edit := analyze.Edit{}

	revisions := historyResp.Query.Pages[0].Revisions

	currentRevId := revisions[0].RevId
	mostRecentRevId := findMostRecentTaggedRev(revisions).RevId

	prevInfo, err := FetchInfoById(mostRecentRevId, locale)
	if err != nil {
		return analyze.Edit{}, err
	}

	edit.Prev = prevInfo

	if currentRevId == mostRecentRevId {
		edit.Curr = prevInfo
	} else {
		currInfo, err := FetchInfoById(currentRevId, locale)
		if err != nil {
			return analyze.Edit{}, err
		}
		edit.Curr = currInfo
	}

	return edit, nil
}

func findMostRecentTaggedRev(revisions []rev) rev {
	for i := 1; i < len(revisions); i++ {
		comment := revisions[i].Comment
		if utils.ContainsAny(comment, possibleTags()) {
			return revisions[i]
		}
	}

	return revisions[len(revisions)-1]
}
