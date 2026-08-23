package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"golang.org/x/sync/errgroup"
)

// deadLetter parks a revision that could not be priced. The reason is folded
// into the error if the parking itself fails, because the cursor is about to
// move past this revision and nothing else will remember it.
func (s *Service) deadLetter(ctx context.Context, locale string, c change, revType entity.RevisionType, reason error) error {
	err := s.dead.Put(ctx, entity.FailedRevision{
		RevID:         c.RevID,
		Locale:        locale,
		PageID:        c.PageID,
		PageTitle:     c.Title,
		WikiUserID:    c.UserID,
		Nickname:      c.User,
		Comment:       c.Comment,
		Type:          revType,
		EditedAt:      c.Timestamp,
		Attempts:      1,
		LastError:     reason.Error(),
		LastAttemptAt: time.Now(),
		Status:        entity.FailedPending,
	})
	if err != nil {
		return fmt.Errorf("dead letter %s/%d (%v): %w", locale, c.RevID, reason, err)
	}

	return nil
}

// Replay has another go at the revisions that failed earlier. It is deliberately
// not part of Sync: Sync runs inside a user request on a budget, and working
// through a backlog is not what that budget is for. Call this on a ticker.
//
// A revision failing again is an expected outcome, not an error -- only
// storage giving way is.
func (s *Service) Replay(ctx context.Context) error {
	pending, err := s.dead.ListPending(ctx, s.cfg.ReplayBatchSize)
	if err != nil {
		return fmt.Errorf("sync: list pending: %w", err)
	}

	errs := make([]error, len(pending))

	var g errgroup.Group
	g.SetLimit(s.cfg.Concurrency)

	for i, f := range pending {
		g.Go(func() error {
			if err := s.replayOne(ctx, f); err != nil {
				errs[i] = err
			}

			return nil
		})
	}
	_ = g.Wait()

	return errors.Join(errs...)
}

func (s *Service) replayOne(ctx context.Context, f entity.FailedRevision) error {
	reason := s.attempt(ctx, f.Locale, changeFromFailed(f), f.Type)
	if reason == nil {
		if err := s.dead.Resolve(ctx, f.Locale, f.RevID); err != nil {
			return fmt.Errorf("sync: resolve %s/%d: %w", f.Locale, f.RevID, err)
		}

		return nil
	}

	// Attempts is what the entry carried before this run, so this attempt is
	// the one that may exhaust it.
	if f.Attempts+1 >= s.cfg.MaxAttempts {
		if err := s.dead.Retire(ctx, f.Locale, f.RevID, reason.Error()); err != nil {
			return fmt.Errorf("sync: retire %s/%d: %w", f.Locale, f.RevID, err)
		}

		return nil
	}

	if err := s.dead.Fail(ctx, f.Locale, f.RevID, reason.Error()); err != nil {
		return fmt.Errorf("sync: fail %s/%d: %w", f.Locale, f.RevID, err)
	}

	return nil
}
