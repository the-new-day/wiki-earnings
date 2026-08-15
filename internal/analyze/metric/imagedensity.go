package metric

type ImageDensity struct {
	baseMetric
	densityCap float64
}

func NewImageDensity(weight int, densityCap float64) *ImageDensity {
	return &ImageDensity{
		baseMetric: baseMetric{weight},
		densityCap: densityCap,
	}
}

func (d *ImageDensity) Apply(state ArticleState) float64 {
	wordCount := len(state.Words)
	imageCount := len(state.Images)
	return sat(density(imageCount, wordCount), d.densityCap)
}
