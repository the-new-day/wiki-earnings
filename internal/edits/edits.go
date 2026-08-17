package edits

import "errors"

var (
	ErrPageNotFound = errors.New("page not found")
)

const wikiUrl = "https://wiki.pro-tanki.online/"

const (
	MinorEditTag             = "(ME)"
	ItemAdditionEditTag      = "(IA)"
	ArticleEditTag           = "(AE)"
	RefactoredArticleEditTag = "(RA)"
	TranslatedArticleEditTag = "(TA)"
	NewArticleEditTag        = "(NA)"
)
