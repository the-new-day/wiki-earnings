package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

func taskData(options ...*discordgo.ApplicationCommandInteractionDataOption) discordgo.ApplicationCommandInteractionData {
	return discordgo.ApplicationCommandInteractionData{Name: "task", Options: options}
}

func stringOption(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionString,
		Value: value,
	}
}

func TestOptionalLanguage(t *testing.T) {
	tests := []struct {
		name    string
		data    discordgo.ApplicationCommandInteractionData
		want    entity.Language
		wantErr string
	}{
		{
			name: "an absent option falls back",
			data: taskData(),
			want: entity.LangRU,
		},
		{
			name: "the chosen language is read",
			data: taskData(stringOption("source_lang", "en")),
			want: entity.LangEN,
		},
		{
			name: "an empty option falls back",
			data: taskData(stringOption("source_lang", "")),
			want: entity.LangRU,
		},
		{
			// Discord only sends values the command was registered with, so an
			// unrecognised one means the command definition and this have
			// drifted apart.
			name:    "an unknown language is refused",
			data:    taskData(stringOption("source_lang", "de")),
			wantErr: "unknown language",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := optionalLanguage(tt.data, "source_lang", entity.LangRU)

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Adding a language should reach the command without anybody remembering to
// come back here.
func TestLanguageChoices_CoverEveryLanguage(t *testing.T) {
	choices := languageChoices()

	require.Len(t, choices, len(entity.Languages()))
	assert.Equal(t, "RU", choices[0].Name)
	assert.Equal(t, "ru", choices[0].Value)
	assert.Equal(t, "EN", choices[1].Name)
	assert.Equal(t, "en", choices[1].Value)

	for _, choice := range choices {
		code, ok := choice.Value.(string)
		require.True(t, ok, "choice %q is not a string", choice.Name)

		_, known := entity.ParseLanguage(code)
		assert.True(t, known, "choice %q offers unparsable %q", choice.Name, code)
	}
}
