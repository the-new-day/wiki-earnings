package analyze

type Section struct {
	Level int
	Line  string
}

type Info struct {
	RevId      int
	Title      string
	Words      []string
	Categories []string
	Links      []string
	Images     []string
	Sections   []Section
	Templates  []string
}

func (info *Info) GetMetricScores(metrics ...Metric) []float64 {
	scores := make([]float64, len(metrics))
	for i, metric := range metrics {
		scores[i] = metric.Apply(info) * float64(metric.Weight())
	}
	return scores
}

func (info *Info) GetQuality(metrics ...Metric) float64 {
	totalWeight := 0
	for _, metric := range metrics {
		totalWeight += metric.Weight()
	}

	if totalWeight == 0 {
		return 0
	}

	sum := 0.0
	scores := info.GetMetricScores(metrics...)
	for _, score := range scores {
		sum += score
	}

	return sum / float64(totalWeight)
}

func (info *Info) GetCost(metrics ...Metric) int {
	if len(info.Words) < WordCountTreshold {
		return 0
	}

	return int(info.GetQuality(metrics...) * BaseUnitCost)
}
