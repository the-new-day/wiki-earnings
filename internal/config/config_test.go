package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-new-day/wiki-earnings/internal/config"
	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

// required sets the variables Load refuses to start without, so a test can
// speak only about the ones it cares about.
func required(t *testing.T) {
	t.Helper()

	t.Setenv("DISCORD_BOT_TOKEN", "token")
	t.Setenv("WIKI_ROLE_ID", "1")
	t.Setenv("WIKI_ADMIN_ROLE_ID", "2")
}

func TestLoad_Locales(t *testing.T) {
	required(t)
	t.Setenv("LOCALES", "ru, ua ,en,")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, []string{"ru", "ua", "en"}, cfg.Locales)
}

func TestLoad_LocalesRejectsBadInput(t *testing.T) {
	for name, value := range map[string]string{
		"too long":  "ru,ukr",
		"duplicate": "ru,ua,ru",
		"empty":     ",,",
	} {
		t.Run(name, func(t *testing.T) {
			required(t)
			t.Setenv("LOCALES", value)

			_, err := config.Load()

			assert.Error(t, err)
		})
	}
}

func TestLoad_TaskTargets(t *testing.T) {
	required(t)
	t.Setenv("LOCALES", "ru,ua,en,br")
	t.Setenv("TASK_TARGETS", "ru:ru:100, ua:ru:200, en:en:300, br:en:400")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, map[string]config.TaskTarget{
		"ru": {Language: entity.LangRU, ChannelID: "100"},
		"ua": {Language: entity.LangRU, ChannelID: "200"},
		"en": {Language: entity.LangEN, ChannelID: "300"},
		"br": {Language: entity.LangEN, ChannelID: "400"},
	}, cfg.Discord.TaskTargets)
}

// A locale nobody wants tasks for is left out rather than given a blank channel.
func TestLoad_TaskTargetsMaySkipLocales(t *testing.T) {
	required(t)
	t.Setenv("LOCALES", "ru,ua,en,br")
	t.Setenv("TASK_TARGETS", "ru:ru:100")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Len(t, cfg.Discord.TaskTargets, 1)
	assert.NotContains(t, cfg.Discord.TaskTargets, "ua")
}

func TestLoad_TaskTargetsRejectBadInput(t *testing.T) {
	for name, value := range map[string]string{
		"missing field":   "ru:ru",
		"extra field":     "ru:ru:100:200",
		"unknown locale":  "de:ru:100",
		"unknown lang":    "ru:de:100",
		"no channel":      "ru:ru:",
		"locale twice":    "ru:ru:100, ru:en:200",
		"lang and locale": "ru:100:ru",
	} {
		t.Run(name, func(t *testing.T) {
			required(t)
			t.Setenv("LOCALES", "ru,ua,en,br")
			t.Setenv("TASK_TARGETS", value)

			_, err := config.Load()

			assert.Error(t, err)
		})
	}
}

// Locales sharing a language are translated once, not once each.
func TestTaskLanguages_DedupesInLocaleOrder(t *testing.T) {
	required(t)
	t.Setenv("LOCALES", "ru,ua,en,br")
	t.Setenv("TASK_TARGETS", "br:en:400, ua:ru:200, en:en:300, ru:ru:100")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, []entity.Language{entity.LangRU, entity.LangEN}, cfg.TaskLanguages())
}

func TestTaskLanguages_EmptyWithoutTargets(t *testing.T) {
	required(t)

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Empty(t, cfg.TaskLanguages())
}

// Locales sharing a language each keep their own channel.
func TestTaskChannels_GroupsLocalesByLanguage(t *testing.T) {
	required(t)
	t.Setenv("LOCALES", "ru,ua,en,br")
	t.Setenv("TASK_TARGETS", "ru:ru:100, ua:ru:200, en:en:300, br:en:400")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, map[entity.Language][]string{
		entity.LangRU: {"100", "200"},
		entity.LangEN: {"300", "400"},
	}, cfg.TaskChannels())
}
