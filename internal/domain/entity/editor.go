package entity

type Editor struct {
	EditorID int64  `db:"editor_id"`
	Nickname string `db:"nickname"` // Nickname on the wiki site
}

type EditorAccount struct {
	WikiID   int64  `db:"wiki_id"` // userid on the wiki site
	Locale   string `db:"locale"`
	EditorID int64  `db:"editor_id"`
}
