package metric

import "github.com/the-new-day/protanki-wiki-admin/internal/articleinfo"

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

func (p *WordCount) Apply(info articleinfo.Info) float64 {
	wordCount := len(info.Words)
	return satInt(wordCount, p.cap)
}
