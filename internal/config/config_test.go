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

// credentials adds what Load demands as soon as anything is set up to receive
// tasks.
func credentials(t *testing.T) {
	t.Helper()

	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "account")
	t.Setenv("CLOUDFLARE_API_TOKEN", "token")
}

func TestLoad_Locales(t *testing.T) {
	tests := []struct {
		name    string
		locales string
		want    []string
		wantErr bool
	}{
		{
			name:    "spaces and a trailing comma are ignored",
			locales: "ru, ua ,en,",
			want:    []string{"ru", "ua", "en"},
		},
		{
			name:    "a locale that is not two letters is refused",
			locales: "ru,ukr",
			wantErr: true,
		},
		{
			name:    "a locale listed twice is refused",
			locales: "ru,ua,ru",
			wantErr: true,
		},
		{
			name:    "a list with nothing in it is refused",
			locales: ",,",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			required(t)
			t.Setenv("LOCALES", tt.locales)

			cfg, err := config.Load()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.Locales)
		})
	}
}

func TestLoad_TaskTargets(t *testing.T) {
	tests := []struct {
		name        string
		taskTargets string
		want        map[string]config.TaskTarget
		wantErr     bool
	}{
		{
			name:        "every locale is keyed to its own language and channel",
			taskTargets: "ru:ru:100, ua:ru:200, en:en:300, br:en:400",
			want: map[string]config.TaskTarget{
				"ru": {Locale: "ru", Language: entity.LangRU, ChannelID: "100"},
				"ua": {Locale: "ua", Language: entity.LangRU, ChannelID: "200"},
				"en": {Locale: "en", Language: entity.LangEN, ChannelID: "300"},
				"br": {Locale: "br", Language: entity.LangEN, ChannelID: "400"},
			},
		},
		{
			// A locale nobody wants tasks for is left out rather than given a
			// blank channel.
			name:        "a locale may be left out",
			taskTargets: "ru:ru:100",
			want: map[string]config.TaskTarget{
				"ru": {Locale: "ru", Language: entity.LangRU, ChannelID: "100"},
			},
		},
		{
			name:        "nothing configured is nothing to post to",
			taskTargets: "",
			want:        map[string]config.TaskTarget{},
		},
		{
			name:        "an entry missing a field is refused",
			taskTargets: "ru:ru",
			wantErr:     true,
		},
		{
			name:        "an entry with an extra field is refused",
			taskTargets: "ru:ru:100:200",
			wantErr:     true,
		},
		{
			name:        "a locale outside LOCALES is refused",
			taskTargets: "de:ru:100",
			wantErr:     true,
		},
		{
			name:        "an unknown language is refused",
			taskTargets: "ru:de:100",
			wantErr:     true,
		},
		{
			name:        "an entry without a channel is refused",
			taskTargets: "ru:ru:",
			wantErr:     true,
		},
		{
			name:        "a locale listed twice is refused",
			taskTargets: "ru:ru:100, ru:en:200",
			wantErr:     true,
		},
		{
			name:        "the fields in the wrong order are refused",
			taskTargets: "ru:100:ru",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			required(t)
			credentials(t)
			t.Setenv("LOCALES", "ru,ua,en,br")
			t.Setenv("TASK_TARGETS", tt.taskTargets)

			cfg, err := config.Load()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.Discord.TaskTargets)
		})
	}
}

// The order locales are offered in comes from LOCALES, not from however
// TASK_TARGETS happened to be written or a map happened to be walked.
func TestConfig_OrderedTaskTargets(t *testing.T) {
	tests := []struct {
		name        string
		taskTargets string
		want        []config.TaskTarget
	}{
		{
			name:        "follows the order of LOCALES",
			taskTargets: "br:en:400, ua:ru:200, en:en:300, ru:ru:100",
			want: []config.TaskTarget{
				{Locale: "ru", Language: entity.LangRU, ChannelID: "100"},
				{Locale: "ua", Language: entity.LangRU, ChannelID: "200"},
				{Locale: "en", Language: entity.LangEN, ChannelID: "300"},
				{Locale: "br", Language: entity.LangEN, ChannelID: "400"},
			},
		},
		{
			name:        "skips locales with no target",
			taskTargets: "ru:ru:100, en:en:300",
			want: []config.TaskTarget{
				{Locale: "ru", Language: entity.LangRU, ChannelID: "100"},
				{Locale: "en", Language: entity.LangEN, ChannelID: "300"},
			},
		},
		{
			name:        "empty without targets",
			taskTargets: "",
			want:        []config.TaskTarget{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			required(t)
			credentials(t)
			t.Setenv("LOCALES", "ru,ua,en,br")
			t.Setenv("TASK_TARGETS", tt.taskTargets)

			cfg, err := config.Load()

			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.OrderedTaskTargets())
		})
	}
}

// Task targets without a translator behind them would fail on the first /task,
// so Load says so at startup instead.
func TestLoad_CloudflareCredentials(t *testing.T) {
	tests := []struct {
		name        string
		taskTargets string
		accountID   string
		apiToken    string
		wantErr     string
	}{
		{
			name:        "targets without an account are refused",
			taskTargets: "en:en:100",
			apiToken:    "token",
			wantErr:     "CLOUDFLARE_ACCOUNT_ID",
		},
		{
			name:        "targets without a token are refused",
			taskTargets: "en:en:100",
			accountID:   "account",
			wantErr:     "CLOUDFLARE_API_TOKEN",
		},
		{
			name:        "targets with both are accepted",
			taskTargets: "en:en:100",
			accountID:   "account",
			apiToken:    "token",
		},
		{
			name:        "without targets there is nothing to translate",
			taskTargets: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			required(t)
			t.Setenv("LOCALES", "ru,en")
			t.Setenv("TASK_TARGETS", tt.taskTargets)
			t.Setenv("CLOUDFLARE_ACCOUNT_ID", tt.accountID)
			t.Setenv("CLOUDFLARE_API_TOKEN", tt.apiToken)

			_, err := config.Load()

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}
