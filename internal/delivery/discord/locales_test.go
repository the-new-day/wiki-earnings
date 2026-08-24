package discord

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var configuredLocales = []string{"ru", "ua", "en", "br"}

// values is what picking each suggestion would put in the field.
func values(t *testing.T, raw string) []string {
	t.Helper()

	choices := localeSuggestions(raw, configuredLocales)
	picked := make([]string, 0, len(choices))

	for _, choice := range choices {
		value, ok := choice.Value.(string)
		require.True(t, ok, "choice %q is not a string", choice.Name)

		picked = append(picked, value)
	}

	return picked
}

func TestParseLocales_AcceptsCommasAndSpaces(t *testing.T) {
	assert.Equal(t, []string{"ru", "ua", "en"}, parseLocales("ru, ua en"))
}

func TestParseLocales_NormalisesCaseAndDuplicates(t *testing.T) {
	assert.Equal(t, []string{"ru", "en"}, parseLocales("RU, ru, En"))
}

func TestParseLocales_EmptyMeansEveryLocale(t *testing.T) {
	assert.Empty(t, parseLocales("  ,, "))
}

func TestLocaleSuggestions_OffersAllOfThemFirst(t *testing.T) {
	suggestions := localeSuggestions("", configuredLocales)

	require.NotEmpty(t, suggestions)
	assert.Equal(t, "All locales", suggestions[0].Name)
	assert.Equal(t, "ru, ua, en, br", suggestions[0].Value)
	assert.Equal(t, []string{"ru, ua, en, br", "ru", "ua", "en", "br"}, values(t, ""))
}

func TestLocaleSuggestions_BuildOnWhatIsAlreadyPicked(t *testing.T) {
	assert.Equal(t, []string{"ru, ua", "ru, en", "ru, br"}, values(t, "ru, "))
}

func TestLocaleSuggestions_NarrowToWhatIsBeingTyped(t *testing.T) {
	assert.Equal(t, []string{"ru, br"}, values(t, "ru, b"))
}

func TestLocaleSuggestions_TreatTheLastWordAsUnfinished(t *testing.T) {
	assert.Equal(t, []string{"ru"}, values(t, "ru"))
}

func TestLocaleSuggestions_IgnoreWhatIsAlreadyPicked(t *testing.T) {
	assert.Equal(t, []string{"ru, ua, en", "ru, ua, br"}, values(t, "ru, ua, "))
}

func TestLocaleSuggestions_DropUnknownLocales(t *testing.T) {
	assert.Equal(t, []string{"ru"}, values(t, "de, r"))
}

func TestLocaleSuggestions_EmptyWithoutConfiguredLocales(t *testing.T) {
	assert.Empty(t, localeSuggestions("", nil))
}
