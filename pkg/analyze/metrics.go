package analyze

import "github.com/the-new-day/protanki-wiki-admin/pkg/analyze/metric"

type Metric interface {
	Apply(state metric.ArticleState) float64
	Weight() int
}

func GetScore(state metric.ArticleState, metrics ...Metric) float64 {
	if len(state.Words) < WordCountTreshold {
		return 1.0
	}

	score := 0.0
	for _, metric := range metrics {
		score += metric.Apply(state) * float64(metric.Weight())
	}
	return score
}
