package analyze

func GetQualityDiff(Prev Info, Curr Info, metrics ...Metric) float64 {
	return Curr.GetQuality(metrics...) - Prev.GetQuality(metrics...)
}

func GetCost(Prev Info, Curr Info, metrics ...Metric) int {
	qualityDiff := GetQualityDiff(Prev, Curr, metrics...)
	if qualityDiff < 0 {
		qualityDiff *= -0.3
	}
	return int(qualityDiff * BaseUnitCost)
}
