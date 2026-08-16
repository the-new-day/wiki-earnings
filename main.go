package main

import (
	"fmt"

	"github.com/the-new-day/protanki-wiki-admin/internal/articleinfo"
	"github.com/the-new-day/protanki-wiki-admin/pkg/analyze"
	"github.com/the-new-day/protanki-wiki-admin/pkg/analyze/metric"
)

func main() {
	wordCount := metric.NewWordCount(analyze.WordCountWeight, analyze.WordCountCap)
	linkDensity := metric.NewLinkDensity(analyze.LinkDensityWeight, analyze.LinkDensityCap)
	imageDensity := metric.NewImageDensity(analyze.ImageDensityWeight, analyze.ImageDensityCap)
	categoryCount := metric.NewCategoryCount(analyze.CategoryCountWeight, analyze.CategoryCountCap)
	articleStructure := metric.NewArticleStructure(analyze.ArticleStructureWeight, analyze.ArticleStructureSectionCountCap)
	templateUsage := metric.NewTemplateUsage(analyze.TemplateUsageWeight, analyze.TemplateUsageCap)

	metrics := []analyze.Metric{
		wordCount,
		linkDensity,
		imageDensity,
		categoryCount,
		articleStructure,
		templateUsage,
	}

	title := "Ежедневные_подарки"
	locale := "ru"

	info, err := articleinfo.FetchInfo(title, locale)
	cost, err := analyze.GetCost(info, metrics...)

	if err != nil {
		panic(err)
	}

	fmt.Println(cost)

	scores := analyze.GetMetricScores(info, metrics...)
	fmt.Println(scores)
	fmt.Println(analyze.GetQuality(info, metrics...))
}
