package metric

type WordCount struct {
	baseMetric
	wordCap int
}

func NewWordCount(weight int, wordCap int) *WordCount {
	return &WordCount{
		baseMetric: baseMetric{weight},
		wordCap:    wordCap,
	}
}

func (p *WordCount) Apply(state ArticleState) float64 {
	wordCount := len(state.Words)
	return satInt(wordCount, p.wordCap)
}
