package analyze

type Edit struct {
	Prev Info
	Curr Info
}

func (e *Edit) GetQualityDiff(metrics ...Metric) float64 {
	return e.Curr.GetQuality(metrics...) - e.Prev.GetQuality(metrics...)
}

func (e *Edit) GetCost(metrics ...Metric) int {
	qualityDiff := e.GetQualityDiff(metrics...)
	if qualityDiff < 0 {
		qualityDiff *= -0.3
	}
	return int(qualityDiff * BaseUnitCost)
}
