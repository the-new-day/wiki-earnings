package config

import "time"

type Config struct {
	DBPath  string
	Locales []string

	// SyncBatchSize is how many recent changes to ask for at once.
	// MediaWiki caps rclimit at 500 for regular users.
	SyncBatchSize int

	// InitialLookback is how far back to start when a locale has no sync state yet.
	InitialLookback time.Duration

	DeadLetterMaxAttempts int
	DeadLetterBatchSize   int
}

func Default() Config {
	return Config{
		DBPath:                "wiki.db",
		Locales:               []string{"ru", "ua", "en", "br"},
		SyncBatchSize:         500,
		InitialLookback:       30 * 24 * time.Hour,
		DeadLetterMaxAttempts: 5,
		DeadLetterBatchSize:   100,
	}
}
