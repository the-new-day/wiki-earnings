package pricing

import (
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/pricing/metric"
)

// Flat rates for edit types that are not worth scoring.
const (
	MinorEditCost    int64 = 1500
	ItemAdditionCost int64 = 3500
)

// For every %crystalsForPremDay% crystals earned, editors receive 1 additional day of premium.
const CrystalsForPremDay = 20000

// BaseUnitCost is the cost for the work if all metrics are 100%.
const BaseUnitCost = 100000

// TranslatedArticleRate is the share of a full article's cost paid for a
// translation: the text is reproduced rather than authored.
const TranslatedArticleRate = 0.3

// DegradationRate is the share of a quality drop that is still paid out. An
// edit that lowers the metrics is not free work, but it is not full work either.
const DegradationRate = 0.3

// MinPayableWords is the length below which an article is not paid for at all.
const MinPayableWords = 100

// Metric scores one aspect of an article on a 0..1 scale. Weight is how much
// that aspect counts relative to the others.
type Metric interface {
	Apply(info *entity.ArticleInfo) float64
	Weight() int
}

// Pricer turns article metrics into money. It holds the metric set so every
// price in one run is computed against the same yardstick.
type Pricer struct {
	metrics []Metric
}

func New(metrics ...Metric) *Pricer {
	return &Pricer{metrics: metrics}
}

// Default is the metric set the wiki is actually priced with.
func Default() *Pricer {
	return New(
		metric.NewWordCount(metric.DefaultWordCountWeight, metric.DefaultWordCountCap),
		metric.NewLinkDensity(metric.DefaultLinkDensityWeight, metric.DefaultLinkDensityCap),
		metric.NewImageDensity(metric.DefaultImageDensityWeight, metric.DefaultImageDensityCap),
		metric.NewCategoryCount(metric.DefaultCategoryCountWeight, metric.DefaultCategoryCountCap),
		metric.NewArticleStructure(metric.DefaultArticleStructureWeight, metric.DefaultArticleStructureSectionCountCap),
		metric.NewTemplateUsage(metric.DefaultTemplateUsageWeight, metric.DefaultTemplateUsageCap),
	)
}

// Scores returns the weighted score of every metric, in the order they were
// given. Useful for explaining a price, not for computing one.
func (p *Pricer) Scores(info *entity.ArticleInfo) []float64 {
	scores := make([]float64, len(p.metrics))
	for i, m := range p.metrics {
		scores[i] = m.Apply(info) * float64(m.Weight())
	}
	return scores
}

// Quality is the weighted average of all metrics, on a 0..1 scale.
func (p *Pricer) Quality(info *entity.ArticleInfo) float64 {
	totalWeight := 0
	for _, m := range p.metrics {
		totalWeight += m.Weight()
	}

	if totalWeight == 0 {
		return 0
	}

	sum := 0.0
	for _, score := range p.Scores(info) {
		sum += score
	}

	return sum / float64(totalWeight)
}

// QualityDiff is how much an edit moved the article's quality.
func (p *Pricer) QualityDiff(prev, curr *entity.ArticleInfo) float64 {
	return p.Quality(curr) - p.Quality(prev)
}

// ArticleCost prices an article as it now stands, ignoring how it got there.
func (p *Pricer) ArticleCost(info *entity.ArticleInfo) int64 {
	if len(info.Words) < MinPayableWords {
		return 0
	}

	return int64(p.Quality(info) * BaseUnitCost)
}

// EditCost prices the difference an edit made.
func (p *Pricer) EditCost(prev, curr *entity.ArticleInfo) int64 {
	qualityDiff := p.QualityDiff(prev, curr)
	if qualityDiff < 0 {
		qualityDiff *= -DegradationRate
	}

	return int64(qualityDiff * BaseUnitCost)
}

// Cost prices a revision according to what kind of work it was.
func (p *Pricer) Cost(t entity.RevisionType, prev, curr *entity.ArticleInfo) int64 {
	switch t {
	case entity.MinorEdit:
		return MinorEditCost

	case entity.ItemAddition:
		return ItemAdditionCost

	case entity.NewArticle:
		// Nothing to diff against: pay for the article as it now stands.
		return p.ArticleCost(curr)

	case entity.ArticleEdit, entity.RefactoredArticle:
		return p.EditCost(prev, curr)

	case entity.TranslatedArticle:
		// Same yardstick as a new article, discounted once by the rate.
		return int64(float64(p.ArticleCost(curr)) * TranslatedArticleRate)
	}

	return 0
}

// DaysPremium returns the number of days of premium that the editor receives in addition.
func DaysPremium(crystalsEarned int) int {
	return crystalsEarned / CrystalsForPremDay
}
