package tasks_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/usecase/tasks"
)

type translator struct {
	calls int
	err   error
}

func (t *translator) Translate(_ context.Context, text string, _, targetLang entity.Language) (string, error) {
	t.calls++
	if t.err != nil {
		return "", t.err
	}

	return text + " in " + targetLang.String(), nil
}

type poster struct {
	posted map[string]string
	calls  int
}

func (p *poster) PostTask(_ context.Context, localizedTexts map[string]string) error {
	p.calls++
	p.posted = localizedTexts

	return nil
}

var targets = []tasks.Target{
	{Locale: "ru", Language: entity.LangRU},
	{Locale: "ua", Language: entity.LangRU},
	{Locale: "en", Language: entity.LangEN},
	{Locale: "br", Language: entity.LangEN},
}

func post(t *testing.T, tr *translator, locales []string) *poster {
	t.Helper()

	sent := &poster{}
	uc := tasks.New(tr, sent, targets)

	require.NoError(t, uc.PostTask(context.Background(), "text", entity.LangRU, locales))

	return sent
}

func TestPostTask_NoLocalesMeansAllOfThem(t *testing.T) {
	sent := post(t, &translator{}, nil)

	assert.Equal(t, map[string]string{
		"ru": "text",
		"ua": "text",
		"en": "text in en",
		"br": "text in en",
	}, sent.posted)
}

func TestPostTask_PostsOnlyToTheLocalesAskedFor(t *testing.T) {
	sent := post(t, &translator{}, []string{"en"})

	assert.Equal(t, map[string]string{"en": "text in en"}, sent.posted)
}

func TestPostTask_PostsToASingleLocale(t *testing.T) {
	sent := post(t, &translator{}, []string{"br"})

	assert.Equal(t, map[string]string{"br": "text in en"}, sent.posted)
}

func TestPostTask_TranslatesOncePerLanguage(t *testing.T) {
	tr := &translator{}

	post(t, tr, []string{"en", "br"})

	assert.Equal(t, 1, tr.calls)
}

func TestPostTask_LeavesTheSourceLanguageAlone(t *testing.T) {
	tr := &translator{}

	sent := post(t, tr, []string{"ru", "ua"})

	assert.Equal(t, 0, tr.calls)
	assert.Equal(t, map[string]string{"ru": "text", "ua": "text"}, sent.posted)
}

func TestPostTask_RefusesUnknownLocales(t *testing.T) {
	sent := &poster{}
	uc := tasks.New(&translator{}, sent, targets)

	err := uc.PostTask(context.Background(), "text", entity.LangRU, []string{"ru", "de"})

	assert.ErrorIs(t, err, tasks.ErrUnknownLocale)
	assert.Zero(t, sent.calls)
}

func TestPostTask_UnknownLocaleNamesTheOnesThatWork(t *testing.T) {
	uc := tasks.New(&translator{}, &poster{}, targets)

	err := uc.PostTask(context.Background(), "text", entity.LangRU, []string{"de"})

	require.Error(t, err)
	for _, locale := range []string{"ru", "ua", "en", "br"} {
		assert.Contains(t, err.Error(), locale)
	}
}

func TestPostTask_RefusesWithoutTargets(t *testing.T) {
	uc := tasks.New(&translator{}, &poster{}, nil)

	err := uc.PostTask(context.Background(), "text", entity.LangRU, nil)

	assert.ErrorIs(t, err, tasks.ErrNoTargets)
}
func TestPostTask_FailedTranslationPostsNothing(t *testing.T) {
	sent := &poster{}
	uc := tasks.New(&translator{err: errors.New("out of neurons")}, sent, targets)

	err := uc.PostTask(context.Background(), "text", entity.LangRU, nil)

	assert.ErrorIs(t, err, tasks.ErrTranslationFailure)
	assert.Zero(t, sent.calls)
}

func TestLocales_ListsTargetsInOrder(t *testing.T) {
	uc := tasks.New(&translator{}, &poster{}, targets)

	assert.Equal(t, []string{"ru", "ua", "en", "br"}, uc.Locales())
}

func TestLocales_AreNotSharedWithTheCaller(t *testing.T) {
	uc := tasks.New(&translator{}, &poster{}, targets)

	locales := uc.Locales()
	locales[0] = strings.ToUpper(locales[0])

	assert.Equal(t, "ru", uc.Locales()[0])
}
