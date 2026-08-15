package main

import (
	"fmt"

	"github.com/the-new-day/protanki-wiki-admin/internal/articleinfo"
)

func main() {
	// wordCount := metric.NewWordCount(analyze.WordCountWeight, analyze.WordCountCap)
	// linkDensity := metric.NewLinkDensity(analyze.LinkDensityWeight, analyze.LinkDensityCap)
	// imageDensity := metric.NewImageDensity(analyze.ImageDensityWeight, analyze.ImageDensityCap)
	// categoryCount := metric.NewCategoryCount(analyze.CategoryCountWeight, analyze.CategoryCountCap)

	// state := metric.articleinfo.Info{
	// 	Words:      []string{"hello", "world"},
	// 	Links:      []string{"Новобранец"},
	// 	Images:     []string{"1.jpg"},
	// 	Categories: []string{"FAQ", "FAQ", "FAQ", "FAQ", "FAQ", "FAQ", "FAQ", "FAQ", "FAQ", "FAQ", "FAQ"},
	// }

	// fmt.Println(analyze.GetScore(state, wordCount, linkDensity, imageDensity, categoryCount))

	info, err := articleinfo.FetchInfo("Праздничные_акции_и_скидки", "ru")
	if err != nil {
		panic(err)
	}

	fmt.Println(info.Words)
}
