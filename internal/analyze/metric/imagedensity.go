package metric

import (
	"github.com/the-new-day/protanki-wiki-admin/internal/analyze"
	"github.com/the-new-day/protanki-wiki-admin/internal/utils"
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

func (d *ImageDensity) Apply(info *analyze.Info) float64 {
	wordCount := len(info.Words)
	if wordCount < analyze.ImageDensityWordCountTreshold {
		return 0
	}
	imageCount := len(info.Images)
	return utils.Sat(utils.Density(imageCount, wordCount), d.densityCap)
}
