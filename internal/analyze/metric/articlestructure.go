package metric

import (
	"github.com/the-new-day/protanki-wiki-admin/internal/analyze"
	"github.com/the-new-day/protanki-wiki-admin/internal/utils"
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

func (p *ArticleStructure) Apply(info *analyze.Info) float64 {
	if !isSectionStructureValid(info.Sections) {
		return 0
	}

	sectionCount := len(info.Sections)
	return utils.SatInt(sectionCount, p.sectionCountCap)
}

func isSectionStructureValid(sections []analyze.Section) bool {
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
