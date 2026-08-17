package metric

import (
	"github.com/the-new-day/protanki-wiki-admin/internal/analyze"
)

type CategoryCount struct {
	baseMetric
	cap int
}

func NewCategoryCount(weight int, cap int) *CategoryCount {
	return &CategoryCount{
		baseMetric: baseMetric{weight},
		cap:        cap,
	}
}

func (p *CategoryCount) Apply(info *analyze.Info) float64 {
	count := len(info.Categories)
	return analyze.SatInt(count, p.cap)
}
