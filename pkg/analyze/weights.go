package analyze

const (
	WordCountWeight    = 20
	WordCountTreshold  = 100
	WordCountCap       = 5000
	WordCountMidpoint  = 1000
	WordCountSteepness = 0.002

	LinkDensityWeight = 3
	LinkDensityCap    = 4.0

	ImageDensityWeight = 1
	ImageDensityCap    = 0.2

	CategoryCountWeight = 1
	CategoryCountCap    = 3

	ArticleStructureWeight          = 3
	ArticleStructureSectionCountCap = 10

	TemplateUsageWeight = 1
	TemplateUsageCap    = 10
)
