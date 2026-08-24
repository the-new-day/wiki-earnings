package discord

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var configuredLocales = []string{"ru", "ua", "en", "br"}

// suggested is what picking each suggestion in turn would put in the field.
func suggested(t *testing.T, raw string, configured []string) []string {
	t.Helper()

	choices := localeSuggestions(raw, configured)
	values := make([]string, 0, len(choices))

	for _, choice := range choices {
		value, ok := choice.Value.(string)
		require.True(t, ok, "choice %q is not a string", choice.Name)

		values = append(values, value)
	}

	return values
}

func TestParseLocales(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "commas and spaces both separate",
			raw:  "ru, ua en",
			want: []string{"ru", "ua", "en"},
		},
		{
			name: "case is ignored and duplicates collapse",
			raw:  "RU, ru, En",
			want: []string{"ru", "en"},
		},
		{
			name: "separators alone name no locale",
			raw:  "  ,, ",
			want: nil,
		},
		{
			name: "an empty field names no locale",
			raw:  "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseLocales(tt.raw))
		})
	}
}

func TestLocaleSuggestions(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		configured []string
		want       []string
	}{
		{
			name:       "an empty field offers everywhere first, then one locale at a time",
			raw:        "",
			configured: configuredLocales,
			want:       []string{"ru, ua, en, br", "ru", "ua", "en", "br"},
		},
		{
			name:       "suggestions carry the locales already picked",
			raw:        "ru, ",
			configured: configuredLocales,
			want:       []string{"ru, ua", "ru, en", "ru, br"},
		},
		{
			name:       "suggestions narrow to what is being typed",
			raw:        "ru, b",
			configured: configuredLocales,
			want:       []string{"ru, br"},
		},
		{
			name:       "the last word is still being spelled, not settled",
			raw:        "ru",
			configured: configuredLocales,
			want:       []string{"ru"},
		},
		{
			name:       "what is already picked is not offered again",
			raw:        "ru, ua, ",
			configured: configuredLocales,
			want:       []string{"ru, ua, en", "ru, ua, br"},
		},
		{
			name:       "an unknown locale earlier in the field is dropped",
			raw:        "de, r",
			configured: configuredLocales,
			want:       []string{"ru"},
		},
		{
			name:       "nothing to suggest without configured locales",
			raw:        "",
			configured: nil,
			want:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, suggested(t, tt.raw, tt.configured))
		})
	}
}
