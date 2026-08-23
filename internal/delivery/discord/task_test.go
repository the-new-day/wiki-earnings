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

func TestOptionalLanguage_FallsBackWhenAbsent(t *testing.T) {
	lang, err := optionalLanguage(taskData(), "source_lang", entity.LangRU)

	require.NoError(t, err)
	assert.Equal(t, entity.LangRU, lang)
}

func TestOptionalLanguage_ReadsTheChoice(t *testing.T) {
	lang, err := optionalLanguage(taskData(stringOption("source_lang", "en")), "source_lang", entity.LangRU)

	require.NoError(t, err)
	assert.Equal(t, entity.LangEN, lang)
}

func TestOptionalLanguage_RejectsUnknownCode(t *testing.T) {
	_, err := optionalLanguage(taskData(stringOption("source_lang", "de")), "source_lang", entity.LangRU)

	assert.ErrorContains(t, err, "unknown language")
}

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
