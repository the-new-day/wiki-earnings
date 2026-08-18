package metric

import (
	"math"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
)

const (
	DefaultWordCountWeight = 20
	DefaultWordCountCap    = 5000

	// Articles shorter than this score 0: below it there is not enough text to
	// judge, and the logistic curve is normalised to start here.
	wordCountThreshold = 100

	wordCountMidpoint  = 1000
	wordCountSteepness = 0.002
)

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

func (p *WordCount) Apply(info *entity.ArticleInfo) float64 {
	wordCount := len(info.Words)

	lo := logistic(wordCountThreshold)
	hi := logistic(float64(p.cap))
	f := (logistic(float64(wordCount)) - lo) / (hi - lo)
	return math.Max(0, math.Min(1, f))
}

func logistic(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-wordCountSteepness*(x-wordCountMidpoint)))
}
