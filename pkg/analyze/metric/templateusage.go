package metric

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

func (d *TemplateUsage) Apply(state ArticleState) float64 {
	return satInt(len(state.Templates), d.cap)
}
