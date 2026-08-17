package metric

import (
	"github.com/the-new-day/protanki-wiki-admin/internal/analyze"
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

func (d *LinkDensity) Apply(info *analyze.Info) float64 {
	wordCount := len(info.Words)
	if wordCount < analyze.LinkDensityWordCountTreshold {
		return 0
	}

	linkCount := len(info.Links)
	return analyze.Sat(analyze.Density(linkCount, wordCount), d.densityCap)
}
