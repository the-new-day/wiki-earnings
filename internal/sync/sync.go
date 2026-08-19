package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
	"github.com/the-new-day/protanki-wiki-admin/internal/mediawiki"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

// WikiClient is the wiki as this service needs it: a list of what changed, and
// the before/after of one change. It speaks mediawiki's own types rather than
// a private copy of them -- one thin import beats two structs kept in step by
// hand.
type WikiClient interface {
	FetchRecentChanges(ctx context.Context, locale string, since time.Time, limit int) ([]mediawiki.RecentChange, error)
	FetchEdit(ctx context.Context, title string, revID int64, locale string) (mediawiki.Edit, error)
}

// EditorRegistry resolves the wiki account behind an edit to the editor who
// gets paid for it, and is where the two are kept in step. The account --
// (locale, wiki user id) -- is the identity; the nickname is a label attached
// to it and nothing is ever looked up by it.
type EditorRegistry interface {
	// FindByLocaleUser returns the editor a wiki account belongs to, carrying
	// the nickname as stored so that a rename can be spotted. found is false
	// for an account nobody has seen before, which is an ordinary outcome
	// rather than an error.
	FindByLocaleUser(ctx context.Context, locale string, wikiUserID int64) (editor entity.Editor, found bool, err error)

	// Register records a wiki account and the editor behind it. A batch is
	// priced in parallel, so the same new account can arrive twice at once:
	// this has to be idempotent and hand back the same editor both times.
	Register(ctx context.Context, locale string, wikiUserID int64, nickname string) (entity.Editor, error)

	// Rename updates the stored nickname. Nothing is identified by it, so this
	// moves a label and changes nothing else.
	Rename(ctx context.Context, editorID int64, nickname string) error
}

// RevisionWriter stores priced revisions. Upsert, not Insert: the cursor is
// saved per batch, so a crash mid-batch replays edits that are already stored.
type RevisionWriter interface {
	Upsert(ctx context.Context, r entity.Revision) error
}

// StateStore keeps the per-locale cursor.
type StateStore interface {
	// Get returns the zero SyncState for a locale that has never been synced.
	Get(ctx context.Context, locale string) (entity.SyncState, error)
	Save(ctx context.Context, s entity.SyncState) error
}

// DeadLetter holds revisions that could not be priced. The cursor moves past
// them regardless, so this is the only record that they ever existed.
type DeadLetter interface {
	// Put parks a revision. Like every write on the sync path it has to be
	// idempotent, because a replayed batch parks the same failure twice.
	Put(ctx context.Context, f entity.FailedRevision) error
	// ListPending returns entries waiting for another attempt, oldest attempt
	// first.
	ListPending(ctx context.Context, limit int) ([]entity.FailedRevision, error)
	// Fail records another failed attempt and leaves the entry pending.
	Fail(ctx context.Context, locale string, revID int64, reason string) error
	// Retire stops retrying an entry for good.
	Retire(ctx context.Context, locale string, revID int64, reason string) error
	// Resolve removes an entry that finally went through.
	Resolve(ctx context.Context, locale string, revID int64) error
}

// Pricer turns an edit into money. Implemented by domain/pricing.
type Pricer interface {
	Cost(t entity.RevisionType, prev, curr *entity.ArticleInfo) int64
}

// Locker keeps two processes off the same locale. The release function is only
// meaningful when the lock was taken; it is tied to whatever connection holds
// the lock, which is why it comes back as a closure rather than as an Unlock
// method.
type Locker interface {
	TryLock(ctx context.Context, key string) (acquired bool, release func(), err error)
}

// Config tunes one Service. Zero fields fall back to the defaults below, so a
// half-filled Config still runs.
type Config struct {
	Locales []string

	// BatchSize is how many recent changes to ask for at once. MediaWiki caps
	// rclimit at 500 for regular users.
	BatchSize int

	// InitialLookback is how far back a locale starts when it has no cursor.
	InitialLookback time.Duration

	// MinInterval is how long a locale is left alone after a sync. Without it
	// every salary request would hit the wiki.
	MinInterval time.Duration

	// MaxDuration budgets one run. Hitting it is not a failure: the cursor is
	// saved and the next run carries on from there.
	MaxDuration time.Duration

	// Concurrency is how many edits are fetched in parallel within a batch.
	// Each edit costs three round trips, so serial is far too slow.
	Concurrency int

	// MaxAttempts is how often a dead-lettered revision is retried before it
	// is retired.
	MaxAttempts int

	// ReplayBatchSize is how many dead-lettered revisions one Replay takes on.
	ReplayBatchSize int
}

// Defaults for anything Config leaves at zero.
const (
	defaultBatchSize       = 500
	defaultInitialLookback = 30 * 24 * time.Hour
	defaultMinInterval     = time.Minute
	defaultMaxDuration     = 20 * time.Second
	defaultConcurrency     = 8
	defaultMaxAttempts     = 5
	defaultReplayBatchSize = 100
)

func (c Config) withDefaults() Config {
	if c.BatchSize <= 0 {
		c.BatchSize = defaultBatchSize
	}
	if c.InitialLookback <= 0 {
		c.InitialLookback = defaultInitialLookback
	}
	if c.MinInterval <= 0 {
		// Deliberately not "zero means no throttle": forgetting this field
		// would put every salary request straight through to the wiki.
		c.MinInterval = defaultMinInterval
	}
	if c.MaxDuration <= 0 {
		c.MaxDuration = defaultMaxDuration
	}
	if c.Concurrency <= 0 {
		c.Concurrency = defaultConcurrency
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaultMaxAttempts
	}
	if c.ReplayBatchSize <= 0 {
		c.ReplayBatchSize = defaultReplayBatchSize
	}

	return c
}

// Service is the sync itself. It satisfies earnings.Syncer.
type Service struct {
	wiki      WikiClient
	editors   EditorRegistry
	revisions RevisionWriter
	state     StateStore
	dead      DeadLetter
	pricer    Pricer
	locker    Locker
	cfg       Config

	// flight collapses concurrent Sync calls into one. On-demand syncs arrive
	// per request, and the throttle only catches callers that turn up after a
	// run has finished, not the ones that pile up during it.
	flight singleflight.Group
}

// New wires the service. A nil locker means no cross-process locking, which is
// correct for a single instance and merely optimistic for several.
func New(
	wiki WikiClient,
	editors EditorRegistry,
	revisions RevisionWriter,
	state StateStore,
	dead DeadLetter,
	pricer Pricer,
	locker Locker,
	cfg Config,
) *Service {
	if locker == nil {
		locker = noLock{}
	}

	return &Service{
		wiki:      wiki,
		editors:   editors,
		revisions: revisions,
		state:     state,
		dead:      dead,
		pricer:    pricer,
		locker:    locker,
		cfg:       cfg.withDefaults(),
	}
}

// Sync catches every locale up with its wiki. Locales are independent, so one
// failing does not stop the others and every failure comes back joined.
//
// Concurrent calls share a single run, and therefore also its context: if the
// caller that started the run walks away, the ones waiting on it see the
// cancellation. Sync failures are advisory to callers, so that is a fair price
// for not hammering the wiki.
func (s *Service) Sync(ctx context.Context) error {
	_, err, _ := s.flight.Do("sync", func() (any, error) {
		return nil, s.sync(ctx)
	})

	return err
}

func (s *Service) sync(ctx context.Context) error {
	deadline := time.Now().Add(s.cfg.MaxDuration)
	errs := make([]error, len(s.cfg.Locales))

	// Not errgroup.WithContext: a locale that fails must not cancel its
	// siblings. The group is here for the waiting, the errors are collected by
	// hand and joined.
	var g errgroup.Group
	for i, locale := range s.cfg.Locales {
		g.Go(func() error {
			if err := s.syncLocale(ctx, locale, deadline); err != nil {
				errs[i] = fmt.Errorf("sync %s: %w", locale, err)
			}

			return nil
		})
	}
	_ = g.Wait()

	return errors.Join(errs...)
}

func (s *Service) syncLocale(ctx context.Context, locale string, deadline time.Time) error {
	acquired, release, err := s.locker.TryLock(ctx, lockKey(locale))
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	if !acquired {
		// Somebody else is already on this locale, and their result is as good
		// as ours would have been.
		return nil
	}
	defer release()

	state, err := s.state.Get(ctx, locale)
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}

	if s.throttled(state) {
		return nil
	}

	cur := cursor{revID: state.LastRevID, editedAt: state.LastEditedAt}
	if cur.editedAt.IsZero() {
		cur.editedAt = time.Now().Add(-s.cfg.InitialLookback)
	}

	for {
		changes, err := s.wiki.FetchRecentChanges(ctx, locale, cur.editedAt, s.cfg.BatchSize)
		if err != nil {
			return fmt.Errorf("recent changes: %w", err)
		}

		fresh := unseen(changes, cur.revID)
		if err := s.processBatch(ctx, locale, fresh); err != nil {
			return err
		}
		cur = cur.advancedBy(fresh)

		// Saved on every pass, including one that found nothing: the throttle
		// reads updated_at, so a quiet wiki still has to be marked as checked
		// or the next request goes straight back out to it.
		if err := s.save(ctx, locale, cur); err != nil {
			return err
		}

		// No fresh changes means the wiki has nothing past the cursor. It is
		// also what stops a batch the cursor cannot get past from looping
		// forever.
		if len(fresh) == 0 || len(changes) < s.cfg.BatchSize || time.Now().After(deadline) {
			return nil
		}
	}
}

// throttled reports whether this locale was synced recently enough to be left
// alone. A zero UpdatedAt means it has never been synced at all.
func (s *Service) throttled(state entity.SyncState) bool {
	return !state.UpdatedAt.IsZero() && time.Since(state.UpdatedAt) < s.cfg.MinInterval
}

func (s *Service) save(ctx context.Context, locale string, cur cursor) error {
	err := s.state.Save(ctx, entity.SyncState{
		Locale:       locale,
		LastRevID:    cur.revID,
		LastEditedAt: cur.editedAt,
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	return nil
}

// processBatch prices a batch of changes with bounded parallelism. Each edit
// costs three round trips to the wiki, so a serial batch would spend the whole
// run's budget on a handful of them.
func (s *Service) processBatch(ctx context.Context, locale string, changes []mediawiki.RecentChange) error {
	if len(changes) == 0 {
		return nil
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(s.cfg.Concurrency)

	for _, c := range changes {
		g.Go(func() error {
			return s.processChange(gctx, locale, changeFromRecent(c))
		})
	}

	return g.Wait()
}

// processChange handles one edit. Anything that stops it from being priced
// goes to the dead letter rather than up the stack: the cursor moves past this
// revision either way, and one deleted page must not wedge a whole locale.
func (s *Service) processChange(ctx context.Context, locale string, c change) error {
	revType, tagged := classify(c.Comment)
	if !tagged {
		// No tag is a claim not made. Untagged edits are not paid work, and
		// dead-lettering them would bury the real failures in noise.
		return nil
	}

	if c.UserID == 0 {
		// Anonymous edit: there is nobody to pay.
		return nil
	}

	if err := s.attempt(ctx, locale, c, revType); err != nil {
		return s.deadLetter(ctx, locale, c, revType, err)
	}

	return nil
}

// attempt does the actual work for one edit: find who made it, fetch what the
// article looked like before and after, price the difference, store the row. A
// returned error is the reason this revision cannot be priced right now.
func (s *Service) attempt(ctx context.Context, locale string, c change, revType entity.RevisionType) error {
	editorID, err := s.resolveEditor(ctx, locale, c)
	if err != nil {
		return err
	}

	edit, err := s.wiki.FetchEdit(ctx, c.Title, c.RevID, locale)
	if err != nil {
		return fmt.Errorf("fetch edit: %w", err)
	}

	err = s.revisions.Upsert(ctx, entity.Revision{
		RevID:      c.RevID,
		Locale:     locale,
		EditorID:   editorID,
		PageID:     c.PageID,
		PageTitle:  c.Title,
		Type:       revType,
		Comment:    c.Comment,
		Cost:       s.pricer.Cost(revType, &edit.Prev, &edit.Curr),
		EditedAt:   c.Timestamp,
		ComputedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("upsert revision: %w", err)
	}

	return nil
}

// resolveEditor finds who to pay for an edit, and keeps the registry in step
// with the wiki while it is there.
//
// The wiki account is the identity, so an account seen for the first time is
// registered rather than rejected: editors turn up by editing, and refusing to
// pay somebody until an admin has typed their name in is not a rule anyone
// asked for. A nickname that no longer matches is a rename of the same person,
// never a different one -- the account key did not move.
func (s *Service) resolveEditor(ctx context.Context, locale string, c change) (int64, error) {
	editor, found, err := s.editors.FindByLocaleUser(ctx, locale, c.UserID)
	if err != nil {
		return 0, fmt.Errorf("find editor for %s/%d: %w", locale, c.UserID, err)
	}

	if !found {
		editor, err = s.editors.Register(ctx, locale, c.UserID, c.User)
		if err != nil {
			return 0, fmt.Errorf("register %s/%d (%q): %w", locale, c.UserID, c.User, err)
		}

		return editor.EditorID, nil
	}

	// An empty user is the wiki declining to say, not a rename to nothing.
	if c.User != "" && c.User != editor.Nickname {
		if err := s.editors.Rename(ctx, editor.EditorID, c.User); err != nil {
			return 0, fmt.Errorf("rename editor %d to %q: %w", editor.EditorID, c.User, err)
		}
	}

	return editor.EditorID, nil
}

func lockKey(locale string) string {
	return "sync:" + locale
}

// cursor is how far a locale has been read. The timestamp is what the wiki is
// queried with; the revision id is what tells apart the edits sharing it.
type cursor struct {
	revID    int64
	editedAt time.Time
}

// advancedBy moves the cursor past everything in the batch. The wiki returns
// oldest first, but the maximum is taken explicitly rather than on trust.
func (c cursor) advancedBy(changes []mediawiki.RecentChange) cursor {
	for _, ch := range changes {
		if ch.RevID > c.revID {
			c.revID = ch.RevID
		}
		if ch.Timestamp.After(c.editedAt) {
			c.editedAt = ch.Timestamp
		}
	}

	return c
}

// unseen drops the changes already behind the cursor. rcstart is a timestamp
// and inclusive, so every batch after the first opens with edits that were
// already handled; revision ids rise monotonically within one wiki, which is
// what makes a single comparison enough to tell them apart.
func unseen(changes []mediawiki.RecentChange, lastRevID int64) []mediawiki.RecentChange {
	fresh := make([]mediawiki.RecentChange, 0, len(changes))
	for _, c := range changes {
		if c.RevID > lastRevID {
			fresh = append(fresh, c)
		}
	}

	return fresh
}

// noLock stands in for an absent Locker so the sync path has no nil checks.
type noLock struct{}

func (noLock) TryLock(context.Context, string) (bool, func(), error) {
	return true, func() {}, nil
}
