package main

import (
	"fmt"

	"github.com/the-new-day/protanki-wiki-admin/internal/analyze"
	"github.com/the-new-day/protanki-wiki-admin/internal/analyze/metric"
	"github.com/the-new-day/protanki-wiki-admin/internal/edits"
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

	title := "Проп"
	locale := "ru"

	info, err := edits.FetchInfo(title, locale)
	if err != nil {
		panic(err)
	}

	cost := info.GetCost(metrics...)

	fmt.Println(cost)

	scores := info.GetMetricScores(metrics...)
	fmt.Println(scores)
	fmt.Println(info.GetQuality(metrics...))

	// edit, err := parse.FetchLastEdit("Участник:New.Day", "ru")

	// fmt.Println(edit.GetCost(metrics...))
}
