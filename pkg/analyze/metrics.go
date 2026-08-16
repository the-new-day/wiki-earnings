package analyze

import (
	"github.com/the-new-day/protanki-wiki-admin/internal/articleinfo"
)

// BaseUnitCost is the cost for the work if all metrics are 100%.
const BaseUnitCost = 150000

type Metric interface {
	Apply(info articleinfo.Info) float64
	Weight() int
}

func GetMetricScores(info articleinfo.Info, metrics ...Metric) []float64 {
	scores := make([]float64, len(metrics))
	for i, metric := range metrics {
		scores[i] = metric.Apply(info) * float64(metric.Weight())
	}
	return scores
}

func GetQuality(info articleinfo.Info, metrics ...Metric) float64 {
	totalWeight := 0
	for _, metric := range metrics {
		totalWeight += metric.Weight()
	}

	if totalWeight == 0 {
		return 0
	}

	sum := 0.0
	scores := GetMetricScores(info, metrics...)
	for _, score := range scores {
		sum += score
	}

	return sum / float64(totalWeight)
}

func GetCost(info articleinfo.Info, metrics ...Metric) (int, error) {
	if len(info.Words) < WordCountTreshold {
		return 0, nil
	}

	return int(GetQuality(info, metrics...) * BaseUnitCost), nil
}
