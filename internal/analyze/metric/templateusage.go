package metric

import (
	"github.com/the-new-day/protanki-wiki-admin/internal/analyze"
	"github.com/the-new-day/protanki-wiki-admin/internal/utils"
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

func (d *TemplateUsage) Apply(info *analyze.Info) float64 {
	return utils.SatInt(len(info.Templates), d.cap)
}
