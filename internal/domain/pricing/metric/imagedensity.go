package metric

import (
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
)

const (
	DefaultImageDensityWeight = 1
	DefaultImageDensityCap    = 0.2

	imageDensityWordCountThreshold = 300
)

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

func (d *ImageDensity) Apply(info *entity.ArticleInfo) float64 {
	wordCount := len(info.Words)
	if wordCount < imageDensityWordCountThreshold {
		return 0
	}

	imageCount := len(info.Images)
	return Sat(Density(imageCount, wordCount), d.densityCap)
}
