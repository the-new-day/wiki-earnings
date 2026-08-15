package metric

import "github.com/the-new-day/protanki-wiki-admin/internal/articleinfo"

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

func (d *LinkDensity) Apply(info articleinfo.Info) float64 {
	wordCount := len(info.Words)
	linkCount := len(info.Links)
	return sat(density(linkCount, wordCount), d.densityCap)
}
