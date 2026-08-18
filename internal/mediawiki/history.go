package mediawiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
	"golang.org/x/sync/errgroup"
)

const getHistoryQueryString = "/api.php?action=query&prop=revisions&rvlimit=500&rvprop=ids|userid|user|comment|timestamp&format=json&formatversion=2&rvdir=older"

type Edit struct {
	RevID     int64
	Locale    string
	Prev      entity.ArticleInfo
	Curr      entity.ArticleInfo
	Comment   string
	User      string
	UserID    int64
	Timestamp time.Time
	PageID    int64
	PageTitle string
}

func FetchEdit(ctx context.Context, title string, revID int64, locale string) (Edit, error) {
	reqUrl := wikiUrl + locale + getHistoryQueryString + "&rvstartid=" + strconv.FormatInt(revID, 10) + "&titles=" + url.QueryEscape(title)

	body, err := fetch(ctx, reqUrl)
	if err != nil {
		return Edit{}, err
	}

	return parseEdit(ctx, body, revID, locale)
}

type rev struct {
	RevId     int64     `json:"revid"`
	Comment   string    `json:"comment"`
	User      string    `json:"user"`
	UserID    int64     `json:"userid"`
	Timestamp time.Time `json:"timestamp"`
}

type page struct {
	PageId    int64  `json:"pageid"`
	Title     string `json:"title"`
	Revisions []rev  `json:"revisions"`
}

type historyResponse struct {
	Query struct {
		Pages []page `json:"pages"`
	} `json:"query"`
}

func parseEdit(ctx context.Context, jsonResponse []byte, revID int64, locale string) (Edit, error) {
	var op = fmt.Sprintf("fetch edit rev=%d locale=%s", revID, locale)

	var historyResp historyResponse

	if err := json.Unmarshal(jsonResponse, &historyResp); err != nil {
		return Edit{}, fmt.Errorf("%s: json error: %w", op, err)
	}

	if len(historyResp.Query.Pages) == 0 {
		return Edit{}, fmt.Errorf("%s: %w", op, ErrPageNotFound)
	}

	currPage := historyResp.Query.Pages[0]

	if len(currPage.Revisions) == 0 {
		return Edit{}, fmt.Errorf("%s: %w", op, ErrNoRevisions)
	}

	revisions := currPage.Revisions
	currRev := revisions[0]

	edit := Edit{
		RevID:     revID,
		Locale:    locale,
		Comment:   currRev.Comment,
		User:      currRev.User,
		UserID:    currRev.UserID,
		Timestamp: currRev.Timestamp,
		PageID:    currPage.PageId,
		PageTitle: currPage.Title,
		Prev:      entity.ArticleInfo{},
	}

	mostRecentRev, err := findMostRecentTaggedRev(revisions)
	isNewArticle := false

	if errors.Is(err, ErrNoReferencePoint) {
		isNewArticle = true
	} else if err != nil {
		return Edit{}, fmt.Errorf("%s: %w", op, err)
	}

	var currInfo, prevInfo entity.ArticleInfo
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		currInfo, err = FetchInfoByRevId(gctx, revID, locale)
		return err
	})
	g.Go(func() error {
		if isNewArticle {
			prevInfo = entity.ArticleInfo{}
			return nil
		}

		var err error
		prevInfo, err = FetchInfoByRevId(gctx, mostRecentRev.RevId, locale)
		return err
	})
	if err := g.Wait(); err != nil {
		return Edit{}, fmt.Errorf("%s: %w", op, err)
	}
	edit.Curr = currInfo
	edit.Prev = prevInfo

	return edit, nil
}

func findMostRecentTaggedRev(revisions []rev) (rev, error) {
	for i := 1; i < len(revisions); i++ {
		comment := revisions[i].Comment
		if containsAny(comment, possibleTags()) {
			return revisions[i], nil
		}
	}

	return rev{}, ErrNoReferencePoint
}

func containsAny(target string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(target, sub) {
			return true
		}
	}
	return false
}
