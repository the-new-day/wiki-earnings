package metric

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

func (p *ArticleStructure) Apply(state ArticleState) float64 {
	if !isSectionStructureValid(state.Sections) {
		return 0
	}

	sectionCount := len(state.Sections)
	return satInt(sectionCount, p.sectionCountCap)
}

func isSectionStructureValid(sections []Section) bool {
	if len(sections) == 0 || sections[0].Level > 2 {
		return false
	}

	currentLevel := 1
	for _, section := range sections {
		if !inNhood(section.Level, currentLevel, 1) {
			return false
		}

		currentLevel = section.Level
	}

	return true
}
