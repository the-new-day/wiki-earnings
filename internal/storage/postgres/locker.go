package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/the-new-day/protanki-wiki-admin/internal/sync"
)

var _ sync.Locker = (*Locker)(nil)

// Locker keeps two processes off the same locale via a Postgres session-scoped
// advisory lock. The lock lives on one connection, so it needs a held
// connection from the pool rather than a plain query.
type Locker struct {
	pool *pgxpool.Pool
}

func NewLocker(pool *pgxpool.Pool) *Locker {
	return &Locker{
		pool: pool,
	}
}

// TryLock hashes key into the bigint pg_try_advisory_lock takes. The release
// closure runs on a background context, since by the time it is called the
// caller's context may already be done.
func (l *Locker) TryLock(ctx context.Context, key string) (bool, func(), error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("postgres: acquire conn for lock %q: %w", key, err)
	}

	var acquired bool
	err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1, 0))`, key).Scan(&acquired)
	if err != nil {
		conn.Release()
		return false, nil, fmt.Errorf("postgres: try lock %q: %w", key, err)
	}

	if !acquired {
		conn.Release()
		return false, nil, nil
	}

	release := func() {
		var unlocked bool
		_ = conn.QueryRow(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, key).Scan(&unlocked)
		conn.Release()
	}

	return true, release, nil
}
