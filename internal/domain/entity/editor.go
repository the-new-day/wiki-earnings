package entity

type Editor struct {
	EditorID int64
	Nickname string // Nickname on the wiki site

	// PaymentsNickname is the in-game account payments are made to.
	// Empty means payments go to Nickname.
	PaymentsNickname string
}

type EditorAccount struct {
	WikiID   int64 // userid on the wiki site
	Locale   string
	EditorID int64
}
