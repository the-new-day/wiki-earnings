package resync

import (
	"context"
	"fmt"
)

// StateStore drops a locale's cursor.
type StateStore interface {
	Reset(ctx context.Context, locale string) error
}

// DeadLetter drops a locale's parked failures.
type DeadLetter interface {
	Clear(ctx context.Context, locale string) error
}

// Syncer runs the sync itself, once state has been cleared.
type Syncer interface {
	Sync(ctx context.Context) error
}

type UseCase struct {
	state   StateStore
	dead    DeadLetter
	syncer  Syncer
	locales []string
}

func New(state StateStore, dead DeadLetter, syncer Syncer, locales []string) *UseCase {
	return &UseCase{
		state:   state,
		dead:    dead,
		syncer:  syncer,
		locales: locales,
	}
}

func (u *UseCase) Resync(ctx context.Context) error {
	for _, locale := range u.locales {
		if err := u.state.Reset(ctx, locale); err != nil {
			return fmt.Errorf("resync: reset state %s: %w", locale, err)
		}

		if err := u.dead.Clear(ctx, locale); err != nil {
			return fmt.Errorf("resync: clear dead letters %s: %w", locale, err)
		}
	}

	if err := u.syncer.Sync(ctx); err != nil {
		return fmt.Errorf("resync: sync: %w", err)
	}

	return nil
}
