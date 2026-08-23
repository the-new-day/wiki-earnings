package tasks

import (
	"context"
	"errors"
	"fmt"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

var ErrTranslationFailure = errors.New("failed to translate")
var ErrPostingFailure = errors.New("failed to post")

type Translator interface {
	Translate(ctx context.Context, text string, sourceLang, targetLang entity.Language) (string, error)
}

type TaskPoster interface {
	PostTask(ctx context.Context, localizedTexts map[entity.Language]string) error
}

type UseCase struct {
	translator Translator
	taskPoster TaskPoster
	langList   []entity.Language
}

func New(translator Translator, taskPoster TaskPoster, langList []entity.Language) *UseCase {
	return &UseCase{
		translator: translator,
		taskPoster: taskPoster,
		langList:   langList,
	}
}

func (u *UseCase) PostTask(ctx context.Context, text string, sourceLang entity.Language) error {
	translations := map[entity.Language]string{}

	for _, lang := range u.langList {
		if lang == sourceLang {
			translations[sourceLang] = text
			continue
		}

		translation, err := u.translator.Translate(ctx, text, sourceLang, lang)
		if err != nil {
			return fmt.Errorf("%w into %s: %w", ErrTranslationFailure, lang, err)
		}
		translations[lang] = translation
	}

	if err := u.taskPoster.PostTask(ctx, translations); err != nil {
		return fmt.Errorf("%w: %w", ErrPostingFailure, err)
	}

	return nil
}
