package entity

type Editor struct {
	EditorID int64 `db:"editor_id"`
	Nickname int64 `db:"nickname"`
}

type EditorAccount struct {
	Locale   int64 `db:"locale"`
	WikiID   int64 `db:"wiki_id"`
	EditorID int64 `db:"editor_id"`
}
