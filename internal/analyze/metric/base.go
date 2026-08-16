package metric

type baseMetric struct {
	weight int
}

func (m *baseMetric) Weight() int {
	return m.weight
}
