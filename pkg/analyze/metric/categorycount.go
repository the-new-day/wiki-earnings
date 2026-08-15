package metric

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

func (p *CategoryCount) Apply(state ArticleState) float64 {
	count := len(state.Categories)
	return satInt(count, p.cap)
}
