package metric

import (
	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

const (
	DefaultArticleStructureWeight          = 2
	DefaultArticleStructureSectionCountCap = 10
)

type ArticleStructure struct {
	baseMetric
	sectionCountCap int
}

func NewArticleStructure(weight int, sectionCountCap int) *ArticleStructure {
	return &ArticleStructure{
		baseMetric:      baseMetric{weight},
		sectionCountCap: sectionCountCap,
	}
}

func (p *ArticleStructure) Apply(info *entity.ArticleInfo) float64 {
	if !isSectionStructureValid(info.Sections) {
		return 0
	}

	sectionCount := len(info.Sections)
	return SatInt(sectionCount, p.sectionCountCap)
}

func isSectionStructureValid(sections []entity.Section) bool {
	if len(sections) == 0 || sections[0].Level > 2 {
		return false
	}

	for i := 1; i < len(sections); i++ {
		prevLevel := sections[i-1].Level
		currLevel := sections[i].Level

		if currLevel < prevLevel {
			continue
		}

		if currLevel > prevLevel+1 {
			return false
		}
	}

	return true
}
