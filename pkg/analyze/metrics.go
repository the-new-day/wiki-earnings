package analyze

import "github.com/the-new-day/protanki-wiki-admin/internal/articleinfo"

type Metric interface {
	Apply(info articleinfo.Info) float64
	Weight() int
}

func GetScore(info articleinfo.Info, metrics ...Metric) float64 {
	if len(info.Words) < WordCountTreshold {
		return 1.0
	}

	score := 0.0
	for _, metric := range metrics {
		score += metric.Apply(info) * float64(metric.Weight())
	}
	return score
}
