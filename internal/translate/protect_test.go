package translate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/translate/mocks"
)

const tanksURL = "https://wiki.pro-tanki.online/ru/Tanks"

// model stands in for the translation backend. It hands the first request to
// answer, which is where a test says what the model did to the placeholders,
// and echoes everything after it - so a run that comes back as the text it
// started as is one that lost nothing.
func model(t *testing.T, answer func(masked string) string) (*mocks.MockTranslator, *int) {
	t.Helper()

	calls := 0
	translator := mocks.NewMockTranslator(t)

	translator.EXPECT().Translate(mock.Anything, mock.Anything, entity.LangRU, entity.LangEN).
		RunAndReturn(func(_ context.Context, text string, _, _ entity.Language) (string, error) {
			calls++
			if calls == 1 && answer != nil {
				return answer(text), nil
			}

			return text, nil
		}).Maybe()

	return translator, &calls
}

func TestProtected_MasksWhatTheModelWouldCorrupt(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantSent string
	}{
		{
			name:     "a link keeps its label up for translation",
			text:     "Обновите [Танки](" + tanksURL + ") сегодня",
			wantSent: "Обновите [Танки]{0} сегодня",
		},
		{
			name:     "a bare link is protected without the sentence punctuation after it",
			text:     "Смотри " + tanksURL + ".",
			wantSent: "Смотри {0}.",
		},
		{
			name:     "a link with its embed suppressed is protected with the brackets",
			text:     "Смотри <" + tanksURL + "> внимательно",
			wantSent: "Смотри {0} внимательно",
		},
		{
			name:     "a user mention is protected",
			text:     "Вопросы — <@123456789>",
			wantSent: "Вопросы — {0}",
		},
		{
			name:     "a nickname mention is protected",
			text:     "Вопросы — <@!123456789>",
			wantSent: "Вопросы — {0}",
		},
		{
			name:     "a role mention is protected",
			text:     "Задача для <@&123456789>",
			wantSent: "Задача для {0}",
		},
		{
			name:     "a channel mention is protected",
			text:     "Пишите в <#987654321> сегодня",
			wantSent: "Пишите в {0} сегодня",
		},
		{
			name:     "a custom emoji is protected",
			text:     "Готово <:tank:123456789> ура",
			wantSent: "Готово {0} ура",
		},
		{
			name:     "an animated emoji is protected",
			text:     "Готово <a:tank:123456789> ура",
			wantSent: "Готово {0} ура",
		},
		{
			name:     "a slash command mention is protected",
			text:     "Запустите </task list:123456789> сегодня",
			wantSent: "Запустите {0} сегодня",
		},
		{
			name:     "a timestamp is protected",
			text:     "Срок <t:1756080000:R> примерно",
			wantSent: "Срок {0} примерно",
		},
		{
			name:     "a mass mention is protected",
			text:     "@everyone нужна помощь",
			wantSent: "{0} нужна помощь",
		},
		{
			name:     "inline code is protected",
			text:     "Шаблон `{{DidYouKnow}}` добавьте",
			wantSent: "Шаблон {0} добавьте",
		},
		{
			name:     "a code block is protected whole",
			text:     "Вот код:\n```\n/givecry tanker 100\n```\nвыполните",
			wantSent: "Вот код:\n{0}\nвыполните",
		},
		{
			name:     "spans are numbered in the order they appear",
			text:     "Смотри [Танки](" + tanksURL + "), пишите <#987> для <@&123>",
			wantSent: "Смотри [Танки]{0}, пишите {1} для {2}",
		},
		{
			name:     "a text with nothing to protect goes as it is",
			text:     "Обновите статью про танки сегодня",
			wantSent: "Обновите статью про танки сегодня",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sent string

			translator, calls := model(t, func(masked string) string {
				sent = masked
				return masked
			})

			got, err := Protect(translator).Translate(context.Background(), tt.text, entity.LangRU, entity.LangEN)

			require.NoError(t, err)
			assert.Equal(t, tt.wantSent, sent, "what the model was asked to translate")
			assert.Equal(t, tt.text, got, "an untouched translation has to come back as the text it started as")
			assert.Equal(t, 1, *calls)
		})
	}
}

func TestProtected_HandlesWhatTheModelDidToThePlaceholders(t *testing.T) {
	// Two spans, so a swap is something other than the text it started as.
	const text = "Смотри [Танки](" + tanksURL + ") и пиши <@123>"

	tests := []struct {
		name string
		// answer is what the model gives back for the masked text.
		answer func(masked string) string
		// wantCalls above one means it gave up on the placeholders and
		// translated around the spans instead.
		wantCalls int
		want      string
	}{
		{
			name:      "an untouched answer is put back as it was",
			answer:    func(masked string) string { return masked },
			wantCalls: 1,
			want:      text,
		},
		{
			name: "spaces inside the braces are tolerated",
			answer: func(masked string) string {
				return strings.ReplaceAll(strings.ReplaceAll(masked, "{0}", "{ 0 }"), "{1}", "{1 }")
			},
			wantCalls: 1,
			want:      text,
		},
		{
			name: "placeholders that changed places still land where they belong",
			answer: func(masked string) string {
				return "{1} и {0}"
			},
			wantCalls: 1,
			want:      "<@123> и (" + tanksURL + ")",
		},
		{
			name: "a dropped placeholder sends it round the spans",
			answer: func(masked string) string {
				return strings.ReplaceAll(masked, "{0}", "")
			},
			wantCalls: 3,
			want:      text,
		},
		{
			name: "a duplicated placeholder sends it round the spans",
			answer: func(masked string) string {
				return masked + " {0}"
			},
			wantCalls: 3,
			want:      text,
		},
		{
			name: "a placeholder nobody sent sends it round the spans",
			answer: func(masked string) string {
				return strings.ReplaceAll(masked, "{1}", "{7}")
			},
			wantCalls: 3,
			want:      text,
		},
		{
			name: "braces turned into something else send it round the spans",
			answer: func(masked string) string {
				return strings.ReplaceAll(masked, "{0}", "[0]")
			},
			wantCalls: 3,
			want:      text,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translator, calls := model(t, tt.answer)

			got, err := Protect(translator).Translate(context.Background(), text, entity.LangRU, entity.LangEN)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantCalls, *calls)
		})
	}
}

func TestProtected_LeavesTheModelOutOfPurePlaceholders(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "nothing but a link", text: tanksURL},
		{name: "nothing but a mention", text: "<@123456789>"},
		{name: "punctuation around a link", text: "-> " + tanksURL + " <-"},
		{name: "nothing at all", text: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Protect(mocks.NewMockTranslator(t)).
				Translate(context.Background(), tt.text, entity.LangRU, entity.LangEN)

			require.NoError(t, err)
			assert.Equal(t, tt.text, got)
		})
	}
}

func TestProtected_ReportsAFailedTranslation(t *testing.T) {
	errModel := errors.New("out of neurons")

	tests := []struct {
		name    string
		text    string
		failOn  int
		wantErr error
	}{
		{
			name:    "the masked text fails",
			text:    "Смотри [Танки](" + tanksURL + ") сегодня",
			failOn:  1,
			wantErr: errModel,
		},
		{
			name:    "a run between the spans fails after the placeholders were lost",
			text:    "Смотри [Танки](" + tanksURL + ") сегодня",
			failOn:  2,
			wantErr: errModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			translator := mocks.NewMockTranslator(t)
			translator.EXPECT().Translate(mock.Anything, mock.Anything, entity.LangRU, entity.LangEN).
				RunAndReturn(func(_ context.Context, text string, _, _ entity.Language) (string, error) {
					calls++
					if calls == tt.failOn {
						return "", errModel
					}

					return strings.ReplaceAll(text, "{0}", ""), nil
				}).Maybe()

			_, err := Protect(translator).Translate(context.Background(), tt.text, entity.LangRU, entity.LangEN)

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
