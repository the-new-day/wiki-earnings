package entity

import "time"

type EditorEarnings struct {
	EditorID         int64
	Nickname         string
	PaymentsNickname string
	Total            int64
}

// PayTo is the account this editor is paid on.
func (e EditorEarnings) PayTo() string {
	if e.PaymentsNickname != "" {
		return e.PaymentsNickname
	}

	return e.Nickname
}

type SyncState struct {
	Locale       string
	LastRevID    int64
	LastEditedAt time.Time
	UpdatedAt    time.Time
}
