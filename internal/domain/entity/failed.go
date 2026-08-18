package entity

import "time"

type FailedRevisionStatus string

const (
	FailedPending   FailedRevisionStatus = "pending"
	FailedPermanent FailedRevisionStatus = "permanent"
)

type FailedRevision struct {
	RevID         int64
	Locale        string
	PageID        int64
	PageTitle     string
	WikiUserID    int64
	Nickname      string
	Comment       string
	Type          RevisionType
	EditedAt      time.Time
	Attempts      int
	LastError     string
	LastAttemptAt time.Time
	Status        FailedRevisionStatus
	CreatedAt     time.Time
}
