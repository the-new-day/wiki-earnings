package pricing_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/domain/pricing"
	"github.com/the-new-day/wiki-earnings/internal/domain/pricing/metric"
)

func noQualityPricer() *pricing.Pricer {
	return pricing.New(metric.NewVolume(metric.DefaultVolumeCap, metric.DefaultVolumeTableCellWeight))
}

func payableInfo() *entity.ArticleInfo {
	return &entity.ArticleInfo{Words: make([]string, 200)}
}

func withDidYouKnow() *entity.ArticleInfo {
	info := payableInfo()
	info.Templates = append(info.Templates, pricing.DidYouKnowTemplate)

	return info
}

func TestPricer_DidYouKnowBonus(t *testing.T) {
	tests := []struct {
		name string
		// The cost of prev -> curr is compared against the cost of the same
		// work without the template, basePrev -> baseCurr.
		revType   entity.RevisionType
		prev      *entity.ArticleInfo
		curr      *entity.ArticleInfo
		basePrev  *entity.ArticleInfo
		baseCurr  *entity.ArticleInfo
		wantBonus bool
	}{
		{
			name:      "a new article carrying the template is paid the bonus",
			revType:   entity.NewArticle,
			curr:      withDidYouKnow(),
			baseCurr:  payableInfo(),
			wantBonus: true,
		},
		{
			name:      "a translated article carrying the template is paid the bonus",
			revType:   entity.TranslatedArticle,
			curr:      withDidYouKnow(),
			baseCurr:  payableInfo(),
			wantBonus: true,
		},
		{
			name:      "an edit that adds the template is paid the bonus",
			revType:   entity.ArticleEdit,
			prev:      payableInfo(),
			curr:      withDidYouKnow(),
			basePrev:  payableInfo(),
			baseCurr:  payableInfo(),
			wantBonus: true,
		},
		{
			name:      "an edit that leaves the template where it was is not paid again",
			revType:   entity.ArticleEdit,
			prev:      withDidYouKnow(),
			curr:      withDidYouKnow(),
			basePrev:  payableInfo(),
			baseCurr:  payableInfo(),
			wantBonus: false,
		},
		{
			name:      "an edit that removes the template is not paid the bonus",
			revType:   entity.ArticleEdit,
			prev:      withDidYouKnow(),
			curr:      payableInfo(),
			basePrev:  payableInfo(),
			baseCurr:  payableInfo(),
			wantBonus: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := noQualityPricer()

			want := p.Cost(tt.revType, tt.basePrev, tt.baseCurr)
			if tt.wantBonus {
				want += pricing.DidYouKnowBonus
			}

			assert.Equal(t, want, p.Cost(tt.revType, tt.prev, tt.curr))
		})
	}
}

func TestPricer_MinPayment(t *testing.T) {
	tiny := &entity.ArticleInfo{Words: []string{"один", "два", "три"}}
	bigger := &entity.ArticleInfo{Words: make([]string, 800)}
	smaller := &entity.ArticleInfo{Words: make([]string, 50)}

	tests := []struct {
		name    string
		revType entity.RevisionType
		prev    *entity.ArticleInfo
		curr    *entity.ArticleInfo
	}{
		{
			name:    "an article with almost no content still earns the floor",
			revType: entity.NewArticle,
			curr:    tiny,
		},
		{
			name:    "an edit that shrinks the article earns nothing beyond the floor",
			revType: entity.ArticleEdit,
			prev:    bigger,
			curr:    smaller,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, int64(pricing.MinPayment), pricing.Default().Cost(tt.revType, tt.prev, tt.curr))
		})
	}
}
