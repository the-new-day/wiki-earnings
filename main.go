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

	// pricer := pricing.Default()
	// cost := pricer.Cost(entity.ArticleEdit, &edit.Prev, &edit.Curr)
	// fmt.Println(cost)
	//
	// info, err := mediawiki.FetchRecentInfo(ctx, title, locale)
	// if err != nil {
	// 	panic(err)
	// }
	//
	// fmt.Println(pricer.Quality(&info), pricer.Scores(&info))
}
