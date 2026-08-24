package discord

import (
	"slices"
	"strings"
	"unicode"

	"github.com/bwmarrin/discordgo"
)

// maxAutocompleteChoices is how many suggestions Discord accepts at once.
const maxAutocompleteChoices = 25

// parseLocales reads the locales option. Commas and spaces work as separators.
func parseLocales(raw string) []string {
	var locales []string
	seen := map[string]bool{}

	for _, locale := range strings.FieldsFunc(strings.ToLower(raw), isLocaleSeparator) {
		if seen[locale] {
			continue
		}

		seen[locale] = true
		locales = append(locales, locale)
	}

	return locales
}

func isLocaleSeparator(r rune) bool {
	return r == ',' || unicode.IsSpace(r)
}

// localeSuggestions completes the locales option one locale at a time: every
// suggestion carries the locales already picked plus one more.
func localeSuggestions(raw string, configured []string) []*discordgo.ApplicationCommandOptionChoice {
	picked, typing := splitSelection(raw)
	picked = onlyKnown(picked, configured)

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, maxAutocompleteChoices)

	if len(picked) == 0 && typing == "" && len(configured) > 0 {
		choices = append(choices, localeChoice("All locales", configured))
	}

	for _, locale := range configured {
		if slices.Contains(picked, locale) || !strings.HasPrefix(locale, typing) {
			continue
		}

		selection := append(slices.Clone(picked), locale)
		choices = append(choices, localeChoice(strings.Join(selection, ", "), selection))

		if len(choices) == maxAutocompleteChoices {
			break
		}
	}

	return choices
}

func localeChoice(name string, locales []string) *discordgo.ApplicationCommandOptionChoice {
	return &discordgo.ApplicationCommandOptionChoice{
		Name:  name,
		Value: strings.Join(locales, ", "),
	}
}

// splitSelection separates the locales already settled on from the one being typed.
func splitSelection(raw string) (picked []string, typing string) {
	locales := parseLocales(raw)
	if len(locales) == 0 {
		return nil, ""
	}

	if strings.TrimRightFunc(raw, isLocaleSeparator) != raw {
		return locales, ""
	}

	return locales[:len(locales)-1], locales[len(locales)-1]
}

// onlyKnown drops what does not name a locale, so that a typo earlier in the
// field does not stop the rest of it from being completed.
func onlyKnown(locales, configured []string) []string {
	known := make([]string, 0, len(locales))

	for _, locale := range locales {
		if slices.Contains(configured, locale) {
			known = append(known, locale)
		}
	}

	return known
}
