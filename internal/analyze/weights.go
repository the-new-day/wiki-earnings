package analyze

const (
	WordCountWeight    = 20
	WordCountTreshold  = 100
	WordCountCap       = 5000
	WordCountMidpoint  = 1000
	WordCountSteepness = 0.002

	LinkDensityWordCountTreshold = 500
	LinkDensityWeight            = 2
	LinkDensityCap               = 4.0

	ImageDensityWordCountTreshold = 300
	ImageDensityWeight            = 1
	ImageDensityCap               = 0.2

	CategoryCountWeight = 1
	CategoryCountCap    = 1

	ArticleStructureWeight          = 2
	ArticleStructureSectionCountCap = 10

	TemplateUsageWeight = 1
	TemplateUsageCap    = 10
)
