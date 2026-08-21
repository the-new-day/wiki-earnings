package pricing

import (
	"strings"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/pricing/metric"
)

// Flat rates for edit types that are not worth scoring.
const (
	MinorEditCost    int64 = 1500
	ItemAdditionCost int64 = 3500
)

// MinPayment is a minimum payments for all non-minor edits.
const MinPayment = 3500

// For every %crystalsForPremDay% crystals earned, editors receive 1 additional day of premium.
const CrystalsForPremDay = 20000

// BaseUnitCost is the cost for the work if all metrics are 100%.
const BaseUnitCost = 100000

// TranslatedArticleRate is the share of a full article's cost paid for a
// translation: the text is reproduced rather than authored.
const TranslatedArticleRate = 0.3

// QualityFloor is the minimum share of an article's volume-based price that
// is paid regardless of quality.
const QualityFloor = 0.191

// VolumeEditShare and QualityEditShare split how an edit's diff is paid.
const (
	VolumeEditShare  = 0.7
	QualityEditShare = 0.3
)

// DidYouKnowTemplate is the trivia widget's page name, namespace excluded.
const DidYouKnowTemplate = "DidYouKnow"

// DidYouKnowBonus is paid once for adding DidYouKnowTemplate to an article.
const DidYouKnowBonus int64 = 1000

// Metric scores one aspect of an article's quality on a 0..1 scale. Weight is
// how much that aspect counts relative to the others. Content size (word and
// table volume) is deliberately not a Metric: see metric.Volume.
type Metric interface {
	Apply(info *entity.ArticleInfo) float64
	Weight() int
}

// Pricer turns article metrics into money. It holds the metric set so every
// price in one run is computed against the same yardstick.
type Pricer struct {
	volume  *metric.Volume
	metrics []Metric
}

func New(volume *metric.Volume, metrics ...Metric) *Pricer {
	return &Pricer{volume: volume, metrics: metrics}
}

// Default is the metric set the wiki is actually priced with.
func Default() *Pricer {
	return New(
		metric.NewVolume(metric.DefaultVolumeCap, metric.DefaultVolumeTableCellWeight),
		metric.NewLinkDensity(metric.DefaultLinkDensityWeight, metric.DefaultLinkDensityCap),
		metric.NewImageDensity(metric.DefaultImageDensityWeight, metric.DefaultImageDensityCap),
		metric.NewCategoryCount(metric.DefaultCategoryCountWeight, metric.DefaultCategoryCountCap),
		metric.NewArticleStructure(metric.DefaultArticleStructureWeight, metric.DefaultArticleStructureSectionCountCap),
		metric.NewTemplateUsage(metric.DefaultTemplateUsageWeight, metric.DefaultTemplateUsageCap),
		metric.NewTableUsage(metric.DefaultTableUsageWeight, metric.DefaultTableUsageCap),
	)
}

// Scores returns the weighted score of every quality metric, in the order
// they were given. Useful for explaining a price, not for computing one.
func (p *Pricer) Scores(info *entity.ArticleInfo) []float64 {
	scores := make([]float64, len(p.metrics))
	for i, m := range p.metrics {
		scores[i] = m.Apply(info) * float64(m.Weight())
	}
	return scores
}

// Quality is the weighted average of the quality metrics, on a 0..1 scale.
// It does not include content size - see Volume.
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

// Volume is how much content the article has (prose length plus table size),
// on a 0..1 scale.
func (p *Pricer) Volume(info *entity.ArticleInfo) float64 {
	return p.volume.Score(info)
}

// QualityDiff is how much an edit moved the article's quality.
func (p *Pricer) QualityDiff(prev, curr *entity.ArticleInfo) float64 {
	return p.Quality(curr) - p.Quality(prev)
}

// VolumeDiff is how much an edit moved the article's content size.
func (p *Pricer) VolumeDiff(prev, curr *entity.ArticleInfo) float64 {
	return p.Volume(curr) - p.Volume(prev)
}

// ArticleCost prices an article as it now stands, ignoring how it got there.
// Volume sets the scale, quality adjusts it within [QualityFloor, 1] of that
// scale - a large but plain article still gets paid mostly for its size.
func (p *Pricer) ArticleCost(info *entity.ArticleInfo) int64 {
	quality := p.Quality(info)
	multiplier := p.Volume(info) * (QualityFloor + (1-QualityFloor)*quality)
	return int64(multiplier * BaseUnitCost)
}

func (p *Pricer) EditCost(prev, curr *entity.ArticleInfo) int64 {
	payVolume := max(0, p.VolumeDiff(prev, curr)) * VolumeEditShare
	payQuality := max(0, p.QualityDiff(prev, curr)) * QualityEditShare
	return int64((payVolume + payQuality) * BaseUnitCost)
}

// Cost prices a revision according to what kind of work it was.
func (p *Pricer) Cost(t entity.RevisionType, prev, curr *entity.ArticleInfo) int64 {
	switch t {
	case entity.MinorEdit:
		return MinorEditCost

	case entity.ItemAddition:
		return ItemAdditionCost

	case entity.NewArticle:
		return max(MinPayment, p.ArticleCost(curr)) + didYouKnowBonus(curr, nil)

	case entity.ArticleEdit, entity.RefactoredArticle:
		return max(MinPayment, p.EditCost(prev, curr)) + didYouKnowBonus(curr, prev)

	case entity.TranslatedArticle:
		return max(
			MinPayment,
			int64(float64(p.ArticleCost(curr))*TranslatedArticleRate),
		) + didYouKnowBonus(curr, nil)
	}

	return 0
}

func didYouKnowBonus(curr, prev *entity.ArticleInfo) int64 {
	if !hasDidYouKnow(curr) {
		return 0
	}

	if prev != nil && hasDidYouKnow(prev) {
		return 0
	}

	return DidYouKnowBonus
}

func hasDidYouKnow(info *entity.ArticleInfo) bool {
	for _, t := range info.Templates {
		if templateName(t) == DidYouKnowTemplate {
			return true
		}
	}

	return false
}

// templateName strips the namespace prefix off a template title.
func templateName(title string) string {
	_, name, found := strings.Cut(title, ":")
	if !found {
		return title
	}
	return name
}

// DaysPremium returns the number of days of premium that the editor receives in addition.
func DaysPremium(crystalsEarned int) int {
	return crystalsEarned / CrystalsForPremDay
}
