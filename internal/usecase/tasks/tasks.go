package tasks

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

var ErrTranslationFailure = errors.New("failed to translate")
var ErrPostingFailure = errors.New("failed to post")
var ErrUnknownLocale = errors.New("unknown locale")
var ErrNoTargets = errors.New("no locales are set up to receive tasks")

// Target is one locale tasks can be posted to, and the language its editors
// read them in. Several locales commonly share a language.
type Target struct {
	Locale   string
	Language entity.Language
}

type Translator interface {
	Translate(ctx context.Context, text string, sourceLang, targetLang entity.Language) (string, error)
}

type TaskPoster interface {
	// PostTask delivers each locale's text to wherever that locale reads tasks.
	PostTask(ctx context.Context, localizedTexts map[string]string) error
}

type UseCase struct {
	translator Translator
	taskPoster TaskPoster

	// locales keeps the configured order, languages answers what to translate
	// into. Both come from the same targets - one is for listing, one for
	// looking up.
	locales   []string
	languages map[string]entity.Language
}

func New(translator Translator, taskPoster TaskPoster, targets []Target) *UseCase {
	uc := &UseCase{
		translator: translator,
		taskPoster: taskPoster,
		locales:    make([]string, 0, len(targets)),
		languages:  make(map[string]entity.Language, len(targets)),
	}

	for _, target := range targets {
		if _, duplicate := uc.languages[target.Locale]; duplicate {
			continue
		}

		uc.locales = append(uc.locales, target.Locale)
		uc.languages[target.Locale] = target.Language
	}

	return uc
}

// Locales is every locale a task can be sent to, in configured order.
func (u *UseCase) Locales() []string {
	return slices.Clone(u.locales)
}

// PostTask translates text and posts it to the given locales, or to all of them
// when none are named. Locales sharing a language are translated once and get
// the same text.
func (u *UseCase) PostTask(ctx context.Context, text string, sourceLang entity.Language, locales []string) error {
	locales, err := u.resolve(locales)
	if err != nil {
		return err
	}

	translations := map[entity.Language]string{sourceLang: text}
	localizedTexts := make(map[string]string, len(locales))

	for _, locale := range locales {
		lang := u.languages[locale]

		translation, done := translations[lang]
		if !done {
			translation, err = u.translator.Translate(ctx, text, sourceLang, lang)
			if err != nil {
				return fmt.Errorf("%w into %s: %w", ErrTranslationFailure, lang, err)
			}

			translations[lang] = translation
		}

		localizedTexts[locale] = translation
	}

	if err := u.taskPoster.PostTask(ctx, localizedTexts); err != nil {
		return fmt.Errorf("%w: %w", ErrPostingFailure, err)
	}

	return nil
}

// resolve turns a selection into the locales to post to, an empty one meaning
// all of them. An unknown locale is refused rather than skipped.
func (u *UseCase) resolve(locales []string) ([]string, error) {
	if len(u.locales) == 0 {
		return nil, ErrNoTargets
	}

	if len(locales) == 0 {
		return u.locales, nil
	}

	for _, locale := range locales {
		if _, ok := u.languages[locale]; !ok {
			return nil, fmt.Errorf("%w %q, pick from: %s", ErrUnknownLocale, locale, strings.Join(u.locales, ", "))
		}
	}

	return locales, nil
}
