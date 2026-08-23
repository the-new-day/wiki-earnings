package metric

import (
	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

const (
	DefaultTemplateUsageWeight = 1
	DefaultTemplateUsageCap    = 10
)

type TemplateUsage struct {
	baseMetric
	cap int
}

func NewTemplateUsage(weight int, cap int) *TemplateUsage {
	return &TemplateUsage{
		baseMetric: baseMetric{weight},
		cap:        cap,
	}
}

func (d *TemplateUsage) Apply(info *entity.ArticleInfo) float64 {
	return SatInt(len(info.Templates), d.cap)
}
