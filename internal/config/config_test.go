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

// tasks adds the credentials Load demands as soon as anything is set up to
// receive tasks.
func tasks(t *testing.T) {
	t.Helper()

	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "account")
	t.Setenv("CLOUDFLARE_API_TOKEN", "token")
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
	tasks(t)
	t.Setenv("LOCALES", "ru,ua,en,br")
	t.Setenv("TASK_TARGETS", "ru:ru:100, ua:ru:200, en:en:300, br:en:400")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, map[string]config.TaskTarget{
		"ru": {Locale: "ru", Language: entity.LangRU, ChannelID: "100"},
		"ua": {Locale: "ua", Language: entity.LangRU, ChannelID: "200"},
		"en": {Locale: "en", Language: entity.LangEN, ChannelID: "300"},
		"br": {Locale: "br", Language: entity.LangEN, ChannelID: "400"},
	}, cfg.Discord.TaskTargets)
}

// A locale nobody wants tasks for is left out rather than given a blank channel.
func TestLoad_TaskTargetsMaySkipLocales(t *testing.T) {
	required(t)
	tasks(t)
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

// The order locales are offered in comes from LOCALES, not from however
// TASK_TARGETS happened to be written or a map happened to be walked.
func TestOrderedTaskTargets_FollowsLocaleOrder(t *testing.T) {
	required(t)
	tasks(t)
	t.Setenv("LOCALES", "ru,ua,en,br")
	t.Setenv("TASK_TARGETS", "br:en:400, ua:ru:200, en:en:300, ru:ru:100")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, []config.TaskTarget{
		{Locale: "ru", Language: entity.LangRU, ChannelID: "100"},
		{Locale: "ua", Language: entity.LangRU, ChannelID: "200"},
		{Locale: "en", Language: entity.LangEN, ChannelID: "300"},
		{Locale: "br", Language: entity.LangEN, ChannelID: "400"},
	}, cfg.OrderedTaskTargets())
}

// A locale with no target is not offered, so nobody can pick somewhere the task
// has no way of reaching.
func TestOrderedTaskTargets_SkipsLocalesWithoutTargets(t *testing.T) {
	required(t)
	tasks(t)
	t.Setenv("LOCALES", "ru,ua,en,br")
	t.Setenv("TASK_TARGETS", "ru:ru:100, en:en:300")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, []config.TaskTarget{
		{Locale: "ru", Language: entity.LangRU, ChannelID: "100"},
		{Locale: "en", Language: entity.LangEN, ChannelID: "300"},
	}, cfg.OrderedTaskTargets())
}

func TestOrderedTaskTargets_EmptyWithoutTargets(t *testing.T) {
	required(t)

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Empty(t, cfg.OrderedTaskTargets())
}

// Task targets without a translator behind them would fail on the first /task,
// so Load says so at startup instead.
func TestLoad_TaskTargetsNeedCloudflareCredentials(t *testing.T) {
	required(t)
	t.Setenv("LOCALES", "ru,en")
	t.Setenv("TASK_TARGETS", "en:en:100")

	_, err := config.Load()

	assert.ErrorContains(t, err, "CLOUDFLARE_ACCOUNT_ID")
}

// Without task targets there is nothing to translate, so they stay optional.
func TestLoad_NoTaskTargetsNeedsNoCredentials(t *testing.T) {
	required(t)

	_, err := config.Load()

	assert.NoError(t, err)
}
