package analyze

// BaseUnitCost is the cost for the work if all metrics are 100%.
const BaseUnitCost = 100000

type Metric interface {
	Apply(info *Info) float64
	Weight() int
}
