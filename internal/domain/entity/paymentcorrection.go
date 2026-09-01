package entity

import "time"

type PaymentCorrection struct {
	CorrectionID int64
	EditorID     int64
	Amount       int64
	Description  string
	CreatedBy    string
	// CreatedAt is audit only: when the row was entered.
	CreatedAt time.Time
	// AppliesAt is the first instant of the month the correction is booked in,
	// which the monthly report filters on. May be backdated.
	AppliesAt time.Time
}
