package mediawiki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
)

const recentChangesQueryString = "/api.php?action=query&list=recentchanges" +
	"&rcprop=ids|title|timestamp|user|userid|comment&rctype=edit|new" +
	"&rcdir=newer&format=json&formatversion=2"

type RecentChange struct {
	RevID     int64
	PageID    int64
	Title     string
	User      string
	UserID    int64
	Comment   string
	Timestamp time.Time
}

// FetchRecentChanges returns up to limit changes of a locale starting at since
// (inclusive), oldest first. Callers advance their cursor and call again to
// page through; an empty result means there is nothing newer.
func FetchRecentChanges(ctx context.Context, locale string, since time.Time, limit int) ([]entity.RecentChange, error) {
	var op = fmt.Sprintf("fetch recent changes locale=%s since=%s", locale, since.UTC().Format(time.RFC3339))

	reqUrl := WikiUrl + locale + recentChangesQueryString +
		"&rclimit=" + strconv.Itoa(limit) +
		"&rcstart=" + url.QueryEscape(since.UTC().Format(time.RFC3339))

	body, err := fetch(ctx, reqUrl)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	changes, err := parseRecentChanges(body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return changes, nil
}

type recentChange struct {
	RevId     int64     `json:"revid"`
	PageId    int64     `json:"pageid"`
	Title     string    `json:"title"`
	User      string    `json:"user"`
	UserID    int64     `json:"userid"`
	Comment   string    `json:"comment"`
	Timestamp time.Time `json:"timestamp"`
}

type recentChangesResponse struct {
	Query struct {
		RecentChanges []recentChange `json:"recentchanges"`
	} `json:"query"`
	Error struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

func parseRecentChanges(jsonResponse []byte) ([]entity.RecentChange, error) {
	var resp recentChangesResponse

	if err := json.Unmarshal(jsonResponse, &resp); err != nil {
		return nil, fmt.Errorf("json error: %w", err)
	}

	if resp.Error.Code != "" {
		return nil, fmt.Errorf("API error: [%s] %s", resp.Error.Code, resp.Error.Info)
	}

	changes := make([]entity.RecentChange, len(resp.Query.RecentChanges))
	for i, rc := range resp.Query.RecentChanges {
		changes[i] = entity.RecentChange{
			RevID:     rc.RevId,
			PageID:    rc.PageId,
			Title:     rc.Title,
			User:      rc.User,
			UserID:    rc.UserID,
			Comment:   rc.Comment,
			Timestamp: rc.Timestamp,
		}
	}

	return changes, nil
}
