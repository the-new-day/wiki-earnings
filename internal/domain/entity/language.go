package entity

import (
	"fmt"
	"slices"
)

// Language is a language a text is written in. Locales map onto it, many to
// one: several wiki editions are commonly served in the same language, and a
// task is translated once per language rather than once per locale.
type Language int

const (
	LangRU Language = 1
	LangEN Language = 2
)

// languageCodes is the whole set of languages the service can handle, and the
// codes configuration refers to them by.
var languageCodes = map[Language]string{
	LangRU: "ru",
	LangEN: "en",
}

// Languages is every language the service knows, ordered by their constants so
// that anything rendering the list - a set of command choices, say - gets the
// same order every run.
func Languages() []Language {
	langs := make([]Language, 0, len(languageCodes))
	for lang := range languageCodes {
		langs = append(langs, lang)
	}

	slices.Sort(langs)

	return langs
}

// ParseLanguage resolves a language code such as "ru". The bool reports
// whether it is one the service knows.
func ParseLanguage(code string) (Language, bool) {
	for lang, known := range languageCodes {
		if known == code {
			return lang, true
		}
	}

	return 0, false
}

func (l Language) String() string {
	if code, ok := languageCodes[l]; ok {
		return code
	}

	return fmt.Sprintf("language(%d)", int(l))
}
