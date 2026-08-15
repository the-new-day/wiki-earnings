package metric

type WordCount struct {
	baseMetric
	cap int
}

func NewWordCount(weight int, cap int) *WordCount {
	return &WordCount{
		baseMetric: baseMetric{weight},
		cap:        cap,
	}
}

func (p *WordCount) Apply(state ArticleState) float64 {
	wordCount := len(state.Words)
	return satInt(wordCount, p.cap)
}
