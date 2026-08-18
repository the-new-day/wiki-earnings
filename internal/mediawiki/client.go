package mediawiki

import (
	"context"
	"time"
)

// Client exposes the package functions as methods so callers can depend on an
// interface and swap in a fake in tests.
type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) FetchRecentChanges(ctx context.Context, locale string, since time.Time, limit int) ([]RecentChange, error) {
	return FetchRecentChanges(ctx, locale, since, limit)
}

func (c *Client) FetchEdit(ctx context.Context, title string, revID int64, locale string) (Edit, error) {
	return FetchEdit(ctx, title, revID, locale)
}
