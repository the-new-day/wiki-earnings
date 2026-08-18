package metric

import (
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
)

const (
	DefaultLinkDensityWeight = 2
	DefaultLinkDensityCap    = 4.0

	// Short articles are not expected to link anywhere, so they are not judged
	// on it either.
	linkDensityWordCountThreshold = 500
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
