package entity

import "time"

type EditorEarnings struct {
	EditorID int64
	Nickname string
	Total    int64
}

type SyncState struct {
	Locale       string
	LastRevID    int64
	LastEditedAt time.Time
	UpdatedAt    time.Time
}
