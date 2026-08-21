package pricing_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/pricing"
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/pricing/metric"
)

func noQualityPricer() *pricing.Pricer {
	return pricing.New(metric.NewVolume(metric.DefaultVolumeCap, metric.DefaultVolumeTableCellWeight))
}

func payableInfo() entity.ArticleInfo {
	return entity.ArticleInfo{Words: make([]string, 200)}
}

func withDidYouKnow(info entity.ArticleInfo) entity.ArticleInfo {
	info.Templates = append(info.Templates, pricing.DidYouKnowTemplate)
	return info
}

func TestDidYouKnowBonus_NewArticle(t *testing.T) {
	p := noQualityPricer()

	withTemplate := withDidYouKnow(payableInfo())
	withoutTemplate := payableInfo()

	costWith := p.Cost(entity.NewArticle, nil, &withTemplate)
	costWithout := p.Cost(entity.NewArticle, nil, &withoutTemplate)

	assert.Equal(t, costWithout+pricing.DidYouKnowBonus, costWith)
}

func TestDidYouKnowBonus_TranslatedArticle(t *testing.T) {
	p := noQualityPricer()

	withTemplate := withDidYouKnow(payableInfo())
	withoutTemplate := payableInfo()

	costWith := p.Cost(entity.TranslatedArticle, nil, &withTemplate)
	costWithout := p.Cost(entity.TranslatedArticle, nil, &withoutTemplate)

	assert.Equal(t, costWithout+pricing.DidYouKnowBonus, costWith)
}

func TestDidYouKnowBonus_EditAddsTemplate(t *testing.T) {
	p := noQualityPricer()
	prev := payableInfo()
	curr := withDidYouKnow(payableInfo())

	cost := p.Cost(entity.ArticleEdit, &prev, &curr)
	baseline := p.Cost(entity.ArticleEdit, &prev, &prev)

	assert.Equal(t, baseline+pricing.DidYouKnowBonus, cost)
}

func TestDidYouKnowBonus_EditKeepsExistingTemplate_NotPaidAgain(t *testing.T) {
	p := noQualityPricer()
	prev := withDidYouKnow(payableInfo())
	curr := withDidYouKnow(payableInfo())

	cost := p.Cost(entity.ArticleEdit, &prev, &curr)

	assert.Equal(t, int64(pricing.MinPayment), cost)
}

func TestDidYouKnowBonus_EditRemovesTemplate_NoBonus(t *testing.T) {
	p := noQualityPricer()
	prev := withDidYouKnow(payableInfo())
	curr := payableInfo()

	cost := p.Cost(entity.ArticleEdit, &prev, &curr)

	assert.Equal(t, int64(pricing.MinPayment), cost)
}

func TestArticleCost_BelowMinPayableContentUnits_IsZero(t *testing.T) {
	p := pricing.Default()
	tiny := entity.ArticleInfo{Words: []string{"один", "два", "три"}}

	assert.Equal(t, int64(0), p.Cost(entity.NewArticle, nil, &tiny))
}

func TestEditCost_NegativeDiffsEarnNothing_ButFloorStillApplies(t *testing.T) {
	p := pricing.Default()
	bigger := entity.ArticleInfo{Words: make([]string, 800)}
	smaller := entity.ArticleInfo{Words: make([]string, 50)}

	// An edit that shrinks both volume and quality should not be paid a
	// negative-scaled "degradation" fee - it earns nothing on either axis,
	// and the edit floor is all that is paid.
	cost := p.Cost(entity.ArticleEdit, &bigger, &smaller)

	assert.Equal(t, int64(pricing.MinPayment), cost)
}
