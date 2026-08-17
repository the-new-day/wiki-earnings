package main

import (
	"context"
	"fmt"

	"github.com/the-new-day/protanki-wiki-admin/internal/mediawiki"
)

func main() {
	title := "Праздничные_акции_и_скидки"
	locale := "ru"
	revID := int64(16552)

	ctx := context.TODO()

	edit, err := mediawiki.FetchEdit(ctx, title, revID, locale)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(edit.Prev.RevId, edit.Curr.RevId)

	// wordCount := metric.NewWordCount(analyze.WordCountWeight, analyze.WordCountCap)
	// linkDensity := metric.NewLinkDensity(analyze.LinkDensityWeight, analyze.LinkDensityCap)
	// imageDensity := metric.NewImageDensity(analyze.ImageDensityWeight, analyze.ImageDensityCap)
	// categoryCount := metric.NewCategoryCount(analyze.CategoryCountWeight, analyze.CategoryCountCap)
	// articleStructure := metric.NewArticleStructure(analyze.ArticleStructureWeight, analyze.ArticleStructureSectionCountCap)
	// templateUsage := metric.NewTemplateUsage(analyze.TemplateUsageWeight, analyze.TemplateUsageCap)

	// metrics := []analyze.Metric{
	// 	wordCount,
	// 	linkDensity,
	// 	imageDensity,
	// 	categoryCount,
	// 	articleStructure,
	// 	templateUsage,
	// }

	// info, err := mediawiki.FetchRecentInfo(title, locale)
	// if err != nil {
	// 	panic(err)
	// }

	// cost := info.GetCost(metrics...)

	// fmt.Println(cost)

	// scores := info.GetMetricScores(metrics...)
	// fmt.Println(scores)
	// fmt.Println(info.GetQuality(metrics...))

	// edit, err := parse.FetchLastEdit("Участник:New.Day", "ru")

	// fmt.Println(edit.GetCost(metrics...))
}
