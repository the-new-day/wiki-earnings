package tasks_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/usecase/tasks"
	"github.com/the-new-day/wiki-earnings/internal/usecase/tasks/mocks"
)

const taskText = "fix the tanks page"

var (
	errTranslator = errors.New("out of neurons")
	errPoster     = errors.New("channel is gone")

	allTargets = []tasks.Target{
		{Locale: "ru", Language: entity.LangRU},
		{Locale: "ua", Language: entity.LangRU},
		{Locale: "en", Language: entity.LangEN},
		{Locale: "br", Language: entity.LangEN},
	}
)

// deps holds the mocked collaborators. Every mock is built with the *testing.T,
// so an unmet expectation fails the test at cleanup and a call nobody set up
// fails it on the spot.
type deps struct {
	translator *mocks.MockTranslator
	poster     *mocks.MockTaskPoster

	posted map[string]string
}

func newDeps(t *testing.T) *deps {
	t.Helper()

	return &deps{
		translator: mocks.NewMockTranslator(t),
		poster:     mocks.NewMockTaskPoster(t),
	}
}

func (d *deps) useCase(targets []tasks.Target) *tasks.UseCase {
	return tasks.New(d.translator, d.poster, targets)
}

// translatesInto marks the text with the language it was asked for, so a posted
// text says where it came from.
func (d *deps) translatesInto(lang entity.Language) {
	d.translator.EXPECT().Translate(mock.Anything, taskText, entity.LangRU, lang).
		Return(taskText+" in "+lang.String(), nil).Once()
}

func (d *deps) translationFails() {
	d.translator.EXPECT().Translate(mock.Anything, taskText, entity.LangRU, mock.Anything).
		Return("", errTranslator).Once()
}

// accepts records what reached the poster.
func (d *deps) accepts() {
	d.poster.EXPECT().PostTask(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, localizedTexts map[string]string) error {
			d.posted = localizedTexts
			return nil
		}).Once()
}

func (d *deps) postFails() {
	d.poster.EXPECT().PostTask(mock.Anything, mock.Anything).Return(errPoster).Once()
}

func TestUseCase_PostTask(t *testing.T) {
	tests := []struct {
		name            string
		targets         []tasks.Target
		locales         []string
		arrange         func(*deps)
		wantPosted      map[string]string
		wantErr         error
		wantErrMentions []string
	}{
		{
			name: "no locales means all of them",
			arrange: func(d *deps) {
				d.translatesInto(entity.LangEN)
				d.accepts()
			},
			wantPosted: map[string]string{
				"ru": taskText,
				"ua": taskText,
				"en": taskText + " in en",
				"br": taskText + " in en",
			},
		},
		{
			name:    "posts only to the locales asked for",
			locales: []string{"en"},
			arrange: func(d *deps) {
				d.translatesInto(entity.LangEN)
				d.accepts()
			},
			wantPosted: map[string]string{"en": taskText + " in en"},
		},
		{
			name:    "posts to a single locale",
			locales: []string{"br"},
			arrange: func(d *deps) {
				d.translatesInto(entity.LangEN)
				d.accepts()
			},
			wantPosted: map[string]string{"br": taskText + " in en"},
		},
		{
			name:    "locales sharing a language are translated once",
			locales: []string{"en", "br"},
			arrange: func(d *deps) {
				d.translatesInto(entity.LangEN)
				d.accepts()
			},
			wantPosted: map[string]string{
				"en": taskText + " in en",
				"br": taskText + " in en",
			},
		},
		{
			name:    "the language it was written in is not translated",
			locales: []string{"ru", "ua"},
			arrange: func(d *deps) {
				d.accepts()
			},
			wantPosted: map[string]string{"ru": taskText, "ua": taskText},
		},
		{
			name:    "an unknown locale stops the post",
			locales: []string{"ru", "de"},
			arrange: func(d *deps) {},
			wantErr: tasks.ErrUnknownLocale,
			// The locales are nowhere the person running the command can look
			// them up.
			wantErrMentions: []string{"ru", "ua", "en", "br"},
		},
		{
			name:    "no locales are set up to receive tasks",
			targets: []tasks.Target{},
			arrange: func(d *deps) {},
			wantErr: tasks.ErrNoTargets,
		},
		{
			name: "a failed translation posts nothing",
			arrange: func(d *deps) {
				d.translationFails()
			},
			wantErr: tasks.ErrTranslationFailure,
		},
		{
			name:    "a failed post is reported",
			locales: []string{"ru"},
			arrange: func(d *deps) {
				d.postFails()
			},
			wantErr: tasks.ErrPostingFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets := allTargets
			if tt.targets != nil {
				targets = tt.targets
			}

			d := newDeps(t)
			tt.arrange(d)

			err := d.useCase(targets).PostTask(context.Background(), taskText, entity.LangRU, tt.locales)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				for _, mention := range tt.wantErrMentions {
					assert.Contains(t, err.Error(), mention)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPosted, d.posted)
		})
	}
}

func TestUseCase_Locales(t *testing.T) {
	tests := []struct {
		name    string
		targets []tasks.Target
		want    []string
	}{
		{
			name:    "lists the targets in configured order",
			targets: allTargets,
			want:    []string{"ru", "ua", "en", "br"},
		},
		{
			name: "a locale listed twice is kept once",
			targets: []tasks.Target{
				{Locale: "ru", Language: entity.LangRU},
				{Locale: "ru", Language: entity.LangEN},
			},
			want: []string{"ru"},
		},
		{
			name:    "empty without targets",
			targets: nil,
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := newDeps(t).useCase(tt.targets)

			assert.Equal(t, tt.want, uc.Locales())
		})
	}
}

// The list is handed out to be rendered, not to be edited underneath the use case.
func TestUseCase_LocalesAreNotSharedWithTheCaller(t *testing.T) {
	uc := newDeps(t).useCase(allTargets)

	uc.Locales()[0] = strings.ToUpper(uc.Locales()[0])

	assert.Equal(t, "ru", uc.Locales()[0])
}
