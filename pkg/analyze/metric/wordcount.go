package metric

import (
	"math"

	"github.com/the-new-day/protanki-wiki-admin/internal/articleinfo"
	"github.com/the-new-day/protanki-wiki-admin/pkg/analyze"
)

type WordCount struct {
	baseMetric
	cap int
}

func logistic(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-analyze.WordCountSteepness*(x-analyze.WordCountMidpoint)))
}

func NewWordCount(weight int, cap int) *WordCount {
	return &WordCount{
		baseMetric: baseMetric{weight},
		cap:        cap,
	}
}

func (p *WordCount) Apply(info articleinfo.Info) float64 {
	wordCount := len(info.Words)

	lo := logistic(analyze.WordCountTreshold)
	hi := logistic(analyze.WordCountCap)
	f := (logistic(float64(wordCount)) - lo) / (hi - lo)
	return math.Max(0, math.Min(1, f))
}
