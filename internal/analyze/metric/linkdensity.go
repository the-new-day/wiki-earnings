package metric

import (
	"github.com/the-new-day/protanki-wiki-admin/internal/analyze"
	"github.com/the-new-day/protanki-wiki-admin/internal/utils"
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
	linkCount := len(info.Links)
	return utils.Sat(utils.Density(linkCount, wordCount), d.densityCap)
}
