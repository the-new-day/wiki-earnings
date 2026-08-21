package entity

// Section is a heading in an article's table of contents.
type Section struct {
	Level int
	Line  string
}

// ArticleInfo is a snapshot of a single revision of an article, reduced to the
// things pricing looks at. It carries no behaviour: metrics read it, the
// mediawiki layer fills it in.
type ArticleInfo struct {
	RevId      int
	Title      string
	Words      []string
	Categories []string
	Links      []string
	Images     []string
	Sections   []Section
	Templates  []string
	Wikitext   string
}
