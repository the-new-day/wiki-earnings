package metric

import "github.com/the-new-day/protanki-wiki-admin/internal/articleinfo"

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

func (d *TemplateUsage) Apply(info articleinfo.Info) float64 {
	return satInt(len(info.Templates), d.cap)
}
