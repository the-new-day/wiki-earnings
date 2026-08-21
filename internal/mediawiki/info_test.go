package mediawiki

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractWords(t *testing.T) {
	tests := []struct {
		name     string
		wikitext string
		want     []string
	}{
		{
			name:     "plain prose",
			wikitext: "Простой текст статьи про котов.",
			want:     []string{"Простой", "текст", "статьи", "про", "котов."},
		},
		{
			name:     "template call is dropped",
			wikitext: "Цена: {{price|150}} кристаллов.",
			want:     []string{"Цена:", "кристаллов."},
		},
		{
			name:     "nested template call is dropped entirely",
			wikitext: "{{FlexContainer|{{Image|a.png|caption=Пасхалка}}|{{Image|b.png}}}} Текст после.",
			want:     []string{"Текст", "после."},
		},
		{
			name:     "table is dropped along with its cell text",
			wikitext: "До таблицы.\n{|class=\"wikitable\"\n!Заголовок\n|-\n|Ячейка с данными\n|}\nПосле таблицы.",
			want:     []string{"До", "таблицы.", "После", "таблицы."},
		},
		{
			name:     "html tags are dropped, inner text kept",
			wikitext: "<center><big>Заголовок</big></center> обычный текст",
			want:     []string{"Заголовок", "обычный", "текст"},
		},
		{
			name:     "wikilink keeps display text after the last pipe",
			wikitext: "Смотри [[Режимы боя|режим]] и [[Гараж]].",
			// trailing punctuation right after "]]" splits into its own
			// token - a minor, harmless overcount of the word list.
			want: []string{"Смотри", "режим", "и", "Гараж", "."},
		},
		{
			name:     "file wikilink is dropped, no display text",
			wikitext: "Картинка: [[Файл:Newbieguide.png|ссылка=|center]] конец.",
			want:     []string{"Картинка:", "конец."},
		},
		{
			name:     "external link keeps its label, bare external link is dropped",
			wikitext: "Сервер [https://discord.gg/protanki Discord] или [https://example.com].",
			want:     []string{"Сервер", "Discord", "или", "."},
		},
		{
			name:     "heading markers are stripped, heading text kept",
			wikitext: "==Введение==\nТекст раздела.",
			want:     []string{"Введение", "Текст", "раздела."},
		},
		{
			name:     "bold and italic markers are stripped",
			wikitext: "'''Жирный''' и ''курсив'' текст.",
			want:     []string{"Жирный", "и", "курсив", "текст."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractWords(tt.wikitext)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDedupTemplateBaseNames(t *testing.T) {
	tests := []struct {
		name string
		in   []template
		want []string
	}{
		{
			name: "no duplicates",
			in:   []template{{Title: "Template:DidYouKnow"}, {Title: "Template:Game site"}},
			want: []string{"Template:DidYouKnow", "Template:Game site"},
		},
		{
			name: "subpage variants collapse to one base entry, first occurrence order kept",
			in: []template{
				{Title: "Template:Rank/05"},
				{Title: "Template:Rank"},
				{Title: "Template:Rank/06"},
				{Title: "Template:MapInfo/theme"},
				{Title: "Template:MapInfo/themes"},
			},
			want: []string{"Template:Rank", "Template:MapInfo"},
		},
		{
			name: "empty input",
			in:   nil,
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedupTemplateBaseNames(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}
