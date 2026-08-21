package metric

import "regexp"

var (
	tableBlockRe     = regexp.MustCompile(`(?s)\{\|.*?\n\|\}`)
	tableCellLineRe  = regexp.MustCompile(`(?m)^[ \t]*[!|][^\-}]`)
	tableInlineSepRe = regexp.MustCompile(`\|\||!!`)
)

// TableCellCount estimates how many table cells a wikitext contains.
// It is a syntax-level approximation.
func TableCellCount(wikitext string) int {
	count := 0

	for _, table := range tableBlockRe.FindAllString(wikitext, -1) {
		count += len(tableCellLineRe.FindAllString(table, -1))
		count += len(tableInlineSepRe.FindAllString(table, -1))
	}

	return count
}
