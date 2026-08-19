package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
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
	Locales  []string

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

	DeadLetterMaxAttempts int
	DeadLetterBatchSize   int
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
		Locales:               []string{"ru", "ua", "en", "br"},
		SyncBatchSize:         500,
		InitialLookback:       30 * 24 * time.Hour,
		SyncMinInterval:       time.Minute,
		SyncMaxDuration:       20 * time.Second,
		SyncConcurrency:       8,
		DeadLetterMaxAttempts: 5,
		DeadLetterBatchSize:   100,
	}
}

// Load starts from Default and overlays whatever the environment sets. An
// unset variable keeps the default; a set but unparsable one is an error,
// because silently falling back to a default nobody asked for is worse.
func Load() (Config, error) {
	cfg := Default()

	var err error
	str(&cfg.Postgres.Host, "POSTGRES_HOST")
	str(&cfg.Postgres.User, "POSTGRES_USER")
	str(&cfg.Postgres.Password, "POSTGRES_PASSWORD")
	str(&cfg.Postgres.Database, "POSTGRES_DB")
	str(&cfg.Postgres.SSLMode, "POSTGRES_SSLMODE")

	if err = intVar(&cfg.Postgres.Port, "POSTGRES_PORT"); err != nil {
		return Config{}, err
	}
	if err = duration(&cfg.Postgres.ConnectTimeout, "POSTGRES_CONNECT_TIMEOUT"); err != nil {
		return Config{}, err
	}
	if err = intVar(&cfg.SyncBatchSize, "SYNC_BATCH_SIZE"); err != nil {
		return Config{}, err
	}
	if err = duration(&cfg.InitialLookback, "INITIAL_LOOKBACK"); err != nil {
		return Config{}, err
	}
	if err = duration(&cfg.SyncMinInterval, "SYNC_MIN_INTERVAL"); err != nil {
		return Config{}, err
	}
	if err = duration(&cfg.SyncMaxDuration, "SYNC_MAX_DURATION"); err != nil {
		return Config{}, err
	}
	if err = intVar(&cfg.SyncConcurrency, "SYNC_CONCURRENCY"); err != nil {
		return Config{}, err
	}
	if err = intVar(&cfg.DeadLetterMaxAttempts, "DEAD_LETTER_MAX_ATTEMPTS"); err != nil {
		return Config{}, err
	}
	if err = intVar(&cfg.DeadLetterBatchSize, "DEAD_LETTER_BATCH_SIZE"); err != nil {
		return Config{}, err
	}

	if raw, ok := os.LookupEnv("LOCALES"); ok {
		locales := strings.Split(raw, ",")
		for i := range locales {
			locales[i] = strings.TrimSpace(locales[i])
		}
		cfg.Locales = locales
	}

	if v, ok := os.LookupEnv("POSTGRES_MAX_CONNS"); ok {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return Config{}, fmt.Errorf("config: POSTGRES_MAX_CONNS=%q: %w", v, err)
		}
		cfg.Postgres.MaxConns = int32(n)
	}

	return cfg, nil
}

func str(dst *string, key string) {
	if v, ok := os.LookupEnv(key); ok {
		*dst = v
	}
}

func intVar(dst *int, key string) error {
	v, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("config: %s=%q: %w", key, v, err)
	}

	*dst = n
	return nil
}

func duration(dst *time.Duration, key string) error {
	v, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("config: %s=%q: %w", key, v, err)
	}

	*dst = d
	return nil
}
