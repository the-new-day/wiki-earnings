package metric

import (
	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

const (
	DefaultLinkDensityWeight = 2
	DefaultLinkDensityCap    = 8.0

	linkDensityWordCountThreshold = 150
)

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

func (d *LinkDensity) Apply(info *entity.ArticleInfo) float64 {
	wordCount := len(info.Words)
	if wordCount < linkDensityWordCountThreshold {
		return 0
	}

	linkCount := len(info.Links)
	return Sat(Density(linkCount, wordCount), d.densityCap)
}
