package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

type Postgres struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string

	MaxConns        int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
}

// TaskTarget is where one locale's tasks are posted and what language they are
// written in.
type TaskTarget struct {
	Language  entity.Language
	ChannelID string
}

type Discord struct {
	BotToken string

	// WikiRoleID and WikiAdminRoleID gate the bot's slash commands. Wiki Admin
	// implicitly has Wiki access too.
	WikiRoleID      string
	WikiAdminRoleID string

	// TaskTargets is where translated tasks go, per locale. A locale with no
	// entry receives no tasks - that is how one is switched off.
	TaskTargets map[string]TaskTarget
}

// DSN renders the connection string. The password is escaped rather than
// interpolated, so it may contain anything.
func (p Postgres) DSN() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(p.User, p.Password),
		Host:   fmt.Sprintf("%s:%d", p.Host, p.Port),
		Path:   p.Database,
	}

	q := url.Values{}
	q.Set("sslmode", p.SSLMode)
	u.RawQuery = q.Encode()

	return u.String()
}

type Config struct {
	Postgres Postgres
	Discord  Discord

	Locales []string

	// SyncBatchSize is how many recent changes to ask for at once.
	// MediaWiki caps rclimit at 500 for regular users.
	SyncBatchSize int

	// InitialLookback is how far back to start when a locale has no sync state yet.
	InitialLookback time.Duration

	// SyncMinInterval is how long a locale is left alone after a sync. Syncs
	// are triggered per request, so without this every page load hits the wiki.
	SyncMinInterval time.Duration

	// SyncMaxDuration budgets one sync run, which happens inside a user
	// request. Running out of it is not a failure, just a shorter run.
	SyncMaxDuration time.Duration

	// SyncConcurrency is how many edits are fetched in parallel. Each one costs
	// three round trips to the wiki.
	SyncConcurrency int

	// MessageLifetime is the time the bot's message will be present in the chat.
	// Not all commands should support this, only "temporary" ones.
	// Zero lifetime means messages don't get deleted.
	MessageLifetime time.Duration

	DeadLetterMaxAttempts int
	DeadLetterBatchSize   int

	// ReplayInterval is how often dead-lettered revisions are retried. Unlike
	// Sync, Replay is not triggered by requests, so something has to call it
	// on a schedule or failures pile up forever.
	ReplayInterval time.Duration
}

func Default() Config {
	return Config{
		Postgres: Postgres{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Password:        "postgres",
			Database:        "wiki",
			SSLMode:         "disable",
			MaxConns:        10,
			MaxConnLifetime: time.Hour,
			ConnectTimeout:  5 * time.Second,
		},
		Discord: Discord{
			TaskTargets: map[string]TaskTarget{},
		},
		Locales:               []string{"ru", "ua", "en", "br"},
		SyncBatchSize:         500,
		InitialLookback:       30 * 24 * time.Hour,
		SyncMinInterval:       time.Minute,
		SyncMaxDuration:       20 * time.Second,
		SyncConcurrency:       8,
		MessageLifetime:       2 * time.Minute,
		DeadLetterMaxAttempts: 5,
		DeadLetterBatchSize:   100,
		ReplayInterval:        5 * time.Minute,
	}
}

// TaskLanguages is every language tasks have to be translated into, each one
// once - locales commonly share a language. The order follows Locales, so it
// does not shift between runs the way map iteration would.
func (c Config) TaskLanguages() []entity.Language {
	langs := make([]entity.Language, 0, len(c.Discord.TaskTargets))
	seen := make(map[entity.Language]bool, len(c.Discord.TaskTargets))

	for _, locale := range c.Locales {
		target, ok := c.Discord.TaskTargets[locale]
		if !ok || seen[target.Language] {
			continue
		}

		seen[target.Language] = true
		langs = append(langs, target.Language)
	}

	return langs
}

// TaskChannels is the channels each language's task goes to. Locales sharing a
// language share one translation and get a message each, so a language maps to
// as many channels as there are locales behind it. The order follows Locales.
func (c Config) TaskChannels() map[entity.Language][]string {
	channels := make(map[entity.Language][]string, len(c.Discord.TaskTargets))

	for _, locale := range c.Locales {
		target, ok := c.Discord.TaskTargets[locale]
		if !ok {
			continue
		}

		channels[target.Language] = append(channels[target.Language], target.ChannelID)
	}

	return channels
}

// Load starts from Default and overlays whatever the environment sets. An
// unset variable keeps the default; a set but unparsable one is an error.
func Load() (Config, error) {
	// Reads keep their first failure instead of returning it, so this reads as
	// the list of variables it is rather than as a chain of identical checks.
	var e env

	cfg := Default()

	e.str(&cfg.Discord.BotToken, "DISCORD_BOT_TOKEN")
	e.str(&cfg.Discord.WikiRoleID, "WIKI_ROLE_ID")
	e.str(&cfg.Discord.WikiAdminRoleID, "WIKI_ADMIN_ROLE_ID")
	e.duration(&cfg.MessageLifetime, "MESSAGE_LIFETIME")

	e.str(&cfg.Postgres.Host, "POSTGRES_HOST")
	e.str(&cfg.Postgres.User, "POSTGRES_USER")
	e.str(&cfg.Postgres.Password, "POSTGRES_PASSWORD")
	e.str(&cfg.Postgres.Database, "POSTGRES_DB")
	e.str(&cfg.Postgres.SSLMode, "POSTGRES_SSLMODE")
	e.intVar(&cfg.Postgres.Port, "POSTGRES_PORT")
	e.int32Var(&cfg.Postgres.MaxConns, "POSTGRES_MAX_CONNS")
	e.duration(&cfg.Postgres.ConnectTimeout, "POSTGRES_CONNECT_TIMEOUT")
	e.duration(&cfg.Postgres.MaxConnLifetime, "POSTGRES_MAX_CONN_LIFETIME")

	e.intVar(&cfg.SyncBatchSize, "SYNC_BATCH_SIZE")
	e.duration(&cfg.InitialLookback, "INITIAL_LOOKBACK")
	e.duration(&cfg.SyncMinInterval, "SYNC_MIN_INTERVAL")
	e.duration(&cfg.SyncMaxDuration, "SYNC_MAX_DURATION")
	e.intVar(&cfg.SyncConcurrency, "SYNC_CONCURRENCY")

	e.intVar(&cfg.DeadLetterMaxAttempts, "DEAD_LETTER_MAX_ATTEMPTS")
	e.intVar(&cfg.DeadLetterBatchSize, "DEAD_LETTER_BATCH_SIZE")
	e.duration(&cfg.ReplayInterval, "REPLAY_INTERVAL")

	// Locales first: task targets are checked against them.
	e.locales(&cfg.Locales, "LOCALES")
	e.taskTargets(&cfg.Discord.TaskTargets, "TASK_TARGETS", cfg.Locales)

	if e.err != nil {
		return Config{}, e.err
	}

	if cfg.Discord.BotToken == "" {
		return Config{}, fmt.Errorf("config: DISCORD_BOT_TOKEN not set")
	}
	if cfg.Discord.WikiRoleID == "" {
		return Config{}, fmt.Errorf("config: WIKI_ROLE_ID not set")
	}
	if cfg.Discord.WikiAdminRoleID == "" {
		return Config{}, fmt.Errorf("config: WIKI_ADMIN_ROLE_ID not set")
	}

	return cfg, nil
}

// env reads configuration out of the environment, holding on to the first
// failure. Once something has failed the remaining
// reads do nothing, so the whole batch can be checked once at the end.
type env struct {
	err error
}

// read hands the raw value to parse, but only when the variable is set: an
// absent one leaves the destination at whatever default it came with.
func (e *env) read(key string, parse func(raw string) error) {
	if e.err != nil {
		return
	}

	raw, ok := os.LookupEnv(key)
	if !ok {
		return
	}

	if err := parse(raw); err != nil {
		e.err = fmt.Errorf("config: %s=%q: %w", key, raw, err)
	}
}

func (e *env) str(dst *string, key string) {
	e.read(key, func(raw string) error {
		*dst = raw
		return nil
	})
}

func (e *env) intVar(dst *int, key string) {
	e.read(key, func(raw string) error {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return err
		}

		*dst = n
		return nil
	})
}

func (e *env) int32Var(dst *int32, key string) {
	e.read(key, func(raw string) error {
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return err
		}

		*dst = int32(n)
		return nil
	})
}

func (e *env) duration(dst *time.Duration, key string) {
	e.read(key, func(raw string) error {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return err
		}

		*dst = d
		return nil
	})
}

// locales reads a comma separated list of wiki editions.
func (e *env) locales(dst *[]string, key string) {
	e.read(key, func(raw string) error {
		var parsed []string
		seen := map[string]bool{}

		for _, value := range splitTrim(raw) {
			if len(value) != 2 {
				return fmt.Errorf("locale %q must be two letters", value)
			}
			if seen[value] {
				return fmt.Errorf("locale %q listed twice", value)
			}

			seen[value] = true
			parsed = append(parsed, value)
		}

		if len(parsed) == 0 {
			return fmt.Errorf("no locales in it")
		}

		*dst = parsed
		return nil
	})
}

// taskTargets reads a comma separated list of <locale>:<language>:<channel id>
// entries. Keying every field to its locale is what keeps them from drifting:
// with parallel lists, one edit in the wrong place silently posts a locale's
// tasks to another locale's channel, in another locale's language.
//
// A locale that is not listed receives no tasks.
func (e *env) taskTargets(dst *map[string]TaskTarget, key string, known []string) {
	e.read(key, func(raw string) error {
		isKnown := make(map[string]bool, len(known))
		for _, locale := range known {
			isKnown[locale] = true
		}

		targets := map[string]TaskTarget{}

		for _, entry := range splitTrim(raw) {
			locale, target, err := taskTarget(entry)
			if err != nil {
				return err
			}

			if !isKnown[locale] {
				return fmt.Errorf("entry %q: %q is not in LOCALES", entry, locale)
			}
			if _, duplicate := targets[locale]; duplicate {
				return fmt.Errorf("locale %q listed twice", locale)
			}

			targets[locale] = target
		}

		*dst = targets
		return nil
	})
}

func taskTarget(entry string) (locale string, target TaskTarget, err error) {
	fields := strings.Split(entry, ":")
	if len(fields) != 3 {
		return "", TaskTarget{}, fmt.Errorf("entry %q: want <locale>:<language>:<channel id>", entry)
	}

	locale = strings.TrimSpace(fields[0])
	code := strings.TrimSpace(fields[1])
	channelID := strings.TrimSpace(fields[2])

	lang, ok := entity.ParseLanguage(code)
	if !ok {
		return "", TaskTarget{}, fmt.Errorf("entry %q: unknown language %q", entry, code)
	}
	if channelID == "" {
		return "", TaskTarget{}, fmt.Errorf("entry %q: no channel id", entry)
	}

	return locale, TaskTarget{Language: lang, ChannelID: channelID}, nil
}

// splitTrim cuts a comma separated variable into its values, dropping the
// blanks a trailing comma or a line break leaves behind.
func splitTrim(raw string) []string {
	var values []string

	for value := range strings.SplitSeq(raw, ",") {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}

	return values
}
