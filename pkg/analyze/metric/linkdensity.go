package metric

type LinkDensity struct {
	baseMetric
	densityCap float64
}

func NewLinkDensity(weight int, densityCap float64) *LinkDensity {
	return &LinkDensity{
		baseMetric: baseMetric{weight},
		densityCap: densityCap,
	}
}

func (d *LinkDensity) Apply(state ArticleState) float64 {
	wordCount := len(state.Words)
	linkCount := len(state.Links)
	return sat(density(linkCount, wordCount), d.densityCap)
}
