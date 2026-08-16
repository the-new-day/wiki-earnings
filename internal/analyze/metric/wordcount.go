package metric

import (
	"math"

	"github.com/the-new-day/protanki-wiki-admin/internal/analyze"
	"github.com/the-new-day/protanki-wiki-admin/internal/utils"
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

func (p *WordCount) Apply(info *analyze.Info) float64 {
	wordCount := len(info.Words)

	lo := utils.Logistic(analyze.WordCountTreshold)
	hi := utils.Logistic(analyze.WordCountCap)
	f := (utils.Logistic(float64(wordCount)) - lo) / (hi - lo)
	return math.Max(0, math.Min(1, f))
}
