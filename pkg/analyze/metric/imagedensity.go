package metric

import "github.com/the-new-day/protanki-wiki-admin/internal/articleinfo"

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

func (d *ImageDensity) Apply(info articleinfo.Info) float64 {
	wordCount := len(info.Words)
	imageCount := len(info.Images)
	return sat(density(imageCount, wordCount), d.densityCap)
}
