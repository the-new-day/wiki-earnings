package metric

import (
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
)

const (
	DefaultCategoryCountWeight = 1
	DefaultCategoryCountCap    = 1
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

func (p *CategoryCount) Apply(info *entity.ArticleInfo) float64 {
	count := len(info.Categories)
	return SatInt(count, p.cap)
}
