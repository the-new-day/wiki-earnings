package metric

import (
	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

const (
	DefaultTableUsageWeight = 3
	DefaultTableUsageCap    = 8
)

// TableUsage scores whether an article organises its data into tables.
type TableUsage struct {
	baseMetric
	cap int
}

func NewTableUsage(weight int, cap int) *TableUsage {
	return &TableUsage{
		baseMetric: baseMetric{weight},
		cap:        cap,
	}
}

func (t *TableUsage) Apply(info *entity.ArticleInfo) float64 {
	return SatInt(TableCellCount(info.Wikitext), t.cap)
}
