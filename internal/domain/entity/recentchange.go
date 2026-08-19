package entity

import (
	"time"
)

type RecentChange struct {
	RevID     int64
	PageID    int64
	Title     string
	User      string
	UserID    int64
	Comment   string
	Timestamp time.Time
}

type Edit struct {
	RevID     int64
	Locale    string
	Prev      ArticleInfo
	Curr      ArticleInfo
	Comment   string
	User      string
	UserID    int64
	Timestamp time.Time
	PageID    int64
	PageTitle string
}
