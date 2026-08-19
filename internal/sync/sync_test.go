package sync_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
	wikisync "github.com/the-new-day/protanki-wiki-admin/internal/sync"
	"github.com/the-new-day/protanki-wiki-admin/internal/sync/mocks"
)

const locale = "ru"

var (
	errWiki  = errors.New("wiki unreachable")
	errStore = errors.New("storage is down")

	// A fixed point to hang timestamps off, so cursors are comparable.
	firstEdit = time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)

	// The wiki account an edit arrives from, and the editor it is filed under.
	wikiUserID int64 = 42
	editor           = entity.Editor{EditorID: 7, Nickname: "tanker"}
)

// change is one edit as the wiki reports it. Timestamps follow revision ids, so
// a batch is ordered the way MediaWiki orders it: oldest first.
func change(revID int64, comment string) entity.RecentChange {
	return entity.RecentChange{
		RevID:     revID,
		PageID:    100,
		Title:     "Tank",
		User:      editor.Nickname,
		UserID:    wikiUserID,
		Comment:   comment,
		Timestamp: firstEdit.Add(time.Duration(revID) * time.Minute),
	}
}

func anonymous(revID int64, comment string) entity.RecentChange {
	c := change(revID, comment)
	c.UserID = 0
	c.User = "77.88.99.1"

	return c
}

// deps holds the mocked collaborators. Every mock is built with the *testing.T,
// so an unmet expectation fails at cleanup and a call nobody set up fails on the
// spot. That is what lets the tests below assert that a skipped edit really is
// skipped without writing a single "was not called" line.
type deps struct {
	wiki      *mocks.MockWikiClient
	editors   *mocks.MockEditorRegistry
	revisions *mocks.MockRevisionWriter
	state     *mocks.MockStateStore
	dead      *mocks.MockDeadLetter
	pricer    *mocks.MockPricer

	// locker stays nil unless a test is about locking: the service installs its
	// own no-op for that case.
	locker wikisync.Locker

	cfg wikisync.Config
}

func newDeps(t *testing.T) *deps {
	return &deps{
		wiki:      mocks.NewMockWikiClient(t),
		editors:   mocks.NewMockEditorRegistry(t),
		revisions: mocks.NewMockRevisionWriter(t),
		state:     mocks.NewMockStateStore(t),
		dead:      mocks.NewMockDeadLetter(t),
		pricer:    mocks.NewMockPricer(t),
		cfg: wikisync.Config{
			Locales:   []string{locale},
			BatchSize: 2,
			// A nanosecond rather than zero: zero asks for the production
			// default, which would throttle every test to a single run.
			MinInterval:     time.Nanosecond,
			Concurrency:     1,
			MaxAttempts:     3,
			ReplayBatchSize: 10,
		},
	}
}

func (d *deps) service() *wikisync.Service {
	return wikisync.New(d.wiki, d.editors, d.revisions, d.state, d.dead, d.pricer, d.locker, d.cfg)
}

// storedAt is the cursor the service starts from. A zero state is a locale that
// has never been synced.
func (d *deps) storedAt(state entity.SyncState) {
	d.state.EXPECT().Get(mock.Anything, locale).Return(state, nil)
}

// wikiReturns hands out the batches in order, one per call and nothing after
// them, recording the cursor each call was made with.
func (d *deps) wikiReturns(batches ...[]entity.RecentChange) *recorder[time.Time] {
	rec := &recorder[time.Time]{}

	d.wiki.EXPECT().FetchRecentChanges(mock.Anything, locale, mock.Anything, d.cfg.BatchSize).
		RunAndReturn(func(_ context.Context, _ string, since time.Time, _ int) ([]entity.RecentChange, error) {
			rec.add(since)
			if n := rec.len() - 1; n < len(batches) {
				return batches[n], nil
			}

			return nil, nil
		})

	return rec
}

// editorIsKnown is the ordinary case: the account has been seen before and the
// nickname on it has not moved.
func (d *deps) editorIsKnown() {
	d.editors.EXPECT().FindByLocaleUser(mock.Anything, locale, wikiUserID).Return(editor, true, nil)
}

// editorIsNew is an account turning up for the first time.
func (d *deps) editorIsNew() {
	d.editors.EXPECT().FindByLocaleUser(mock.Anything, locale, wikiUserID).
		Return(entity.Editor{}, false, nil)
}

func (d *deps) editIsFetchable() {
	d.wiki.EXPECT().FetchEdit(mock.Anything, mock.Anything, mock.Anything, locale).
		Return(entity.Edit{Curr: entity.ArticleInfo{Title: "Танк"}}, nil)
}

func (d *deps) pricesAt(cost int64) {
	d.pricer.EXPECT().Cost(mock.Anything, mock.Anything, mock.Anything).Return(cost)
}

func (d *deps) saves() *recorder[entity.SyncState] {
	return record(d.state.EXPECT().Save(mock.Anything, mock.Anything).RunAndReturn)
}

func (d *deps) stores() *recorder[entity.Revision] {
	return record(d.revisions.EXPECT().Upsert(mock.Anything, mock.Anything).RunAndReturn)
}

func (d *deps) deadLetters() *recorder[entity.FailedRevision] {
	return record(d.dead.EXPECT().Put(mock.Anything, mock.Anything).RunAndReturn)
}

// record wires a one-argument write mock to a recorder, so assertions can be
// about the values that were written rather than about a call having happened.
// Call is the mock's own chaining type, which differs per interface and is of
// no interest here.
func record[T, Call any](runAndReturn func(func(context.Context, T) error) Call) *recorder[T] {
	rec := &recorder[T]{}
	runAndReturn(func(_ context.Context, v T) error {
		rec.add(v)

		return nil
	})

	return rec
}

type recorder[T any] struct {
	mu    sync.Mutex
	items []T
}

func (r *recorder[T]) add(v T) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items = append(r.items, v)
}

func (r *recorder[T]) all() []T {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.items)
}

func (r *recorder[T]) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.items)
}

func (r *recorder[T]) last(t *testing.T) T {
	t.Helper()

	all := r.all()
	require.NotEmpty(t, all, "nothing was written")

	return all[len(all)-1]
}

func TestService_StoresPricedRevisions(t *testing.T) {
	const cost = 4200

	tests := []struct {
		name     string
		comment  string
		wantType entity.RevisionType
	}{
		{"minor edit", "(ME)", entity.MinorEdit},
		{"item addition", "(IA) added a paint", entity.ItemAddition},
		{"article edit", "(AE) /* Tanks */ added a section", entity.ArticleEdit},
		{"refactored article", "(RA) reworked the structure", entity.RefactoredArticle},
		{"translated article", "(TA) translated from RU", entity.TranslatedArticle},
		{"new article", "(NA) new article!!", entity.NewArticle},
		{"a new article outranks an edit when both are claimed", "(NA) (AE)", entity.NewArticle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edit := change(11, tt.comment)

			d := newDeps(t)
			d.storedAt(entity.SyncState{})
			saved := d.saves()
			d.wikiReturns([]entity.RecentChange{edit})
			d.editorIsKnown()
			d.editIsFetchable()
			stored := d.stores()

			priced := &recorder[entity.RevisionType]{}
			d.pricer.EXPECT().Cost(mock.Anything, mock.Anything, mock.Anything).
				RunAndReturn(func(kind entity.RevisionType, _, _ *entity.ArticleInfo) int64 {
					priced.add(kind)

					return cost
				})

			require.NoError(t, d.service().Sync(context.Background()))

			assert.Equal(t, tt.wantType, priced.last(t), "the tag decides what kind of work was paid for")

			rev := stored.last(t)
			assert.Equal(t, tt.wantType, rev.Type)
			assert.Equal(t, int64(cost), rev.Cost)
			assert.Equal(t, editor.EditorID, rev.EditorID, "the row is filed under the editor, not the wiki account")
			assert.Equal(t, edit.RevID, rev.RevID)
			assert.Equal(t, locale, rev.Locale)
			assert.Equal(t, edit.Timestamp, rev.EditedAt)
			assert.Equal(t, edit.Comment, rev.Comment)

			assert.Equal(t, edit.RevID, saved.last(t).LastRevID, "the cursor follows the last edit read")
		})
	}
}

func TestService_SkipsEditsNobodyCanBePaidFor(t *testing.T) {
	tests := []struct {
		name string
		edit entity.RecentChange
	}{
		{"an untagged edit is not a claim for payment", change(11, "default edit")},
		{"an anonymous edit has nobody to pay", anonymous(11, "(AE) payable edit")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			d.storedAt(entity.SyncState{})
			saved := d.saves()
			d.wikiReturns([]entity.RecentChange{tt.edit})

			require.NoError(t, d.service().Sync(context.Background()))

			assert.Equal(t, tt.edit.RevID, saved.last(t).LastRevID,
				"a skipped edit still has to move the cursor, or it is read again forever")
		})
	}
}

func TestService_KeepsTheEditorRegistryInStepWithTheWiki(t *testing.T) {
	const renamed = "smoky"

	tests := []struct {
		name    string
		user    string
		arrange func(*deps)
	}{
		{
			name: "an account seen for the first time is registered under its nickname",
			user: editor.Nickname,
			arrange: func(d *deps) {
				d.editorIsNew()
				d.editors.EXPECT().Register(mock.Anything, locale, wikiUserID, editor.Nickname).
					Return(editor, nil)
			},
		},
		{
			name: "a known account is paid without the registry being written to",
			user: editor.Nickname,
			// Only the lookup is set up, so a Register or a Rename here fails
			// the test where it happens.
			arrange: func(d *deps) { d.editorIsKnown() },
		},
		{
			name: "a renamed account keeps its editor and takes the new nickname",
			user: renamed,
			arrange: func(d *deps) {
				d.editorIsKnown()
				d.editors.EXPECT().Rename(mock.Anything, editor.EditorID, renamed).Return(nil)
			},
		},
		{
			name:    "a wiki that does not name the user is not a rename to nothing",
			user:    "",
			arrange: func(d *deps) { d.editorIsKnown() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edit := change(11, "(AE) article edit")
			edit.User = tt.user

			d := newDeps(t)
			d.storedAt(entity.SyncState{})
			d.saves()
			d.wikiReturns([]entity.RecentChange{edit})
			d.editIsFetchable()
			d.pricesAt(1000)
			stored := d.stores()
			tt.arrange(d)

			require.NoError(t, d.service().Sync(context.Background()))

			assert.Equal(t, editor.EditorID, stored.last(t).EditorID,
				"the revision is filed under whichever editor the account resolved to")
		})
	}
}

func TestService_DeadLettersWhatItCannotPrice(t *testing.T) {
	tests := []struct {
		name       string
		arrange    func(*deps)
		wantReason string
	}{
		{
			name: "the editor behind a new account cannot be registered",
			arrange: func(d *deps) {
				d.editorIsNew()
				d.editors.EXPECT().Register(mock.Anything, locale, wikiUserID, editor.Nickname).
					Return(entity.Editor{}, errStore)
			},
			wantReason: errStore.Error(),
		},
		{
			name: "a nickname that cannot be updated holds up the revision",
			arrange: func(d *deps) {
				d.editors.EXPECT().FindByLocaleUser(mock.Anything, locale, wikiUserID).
					Return(entity.Editor{EditorID: editor.EditorID, Nickname: "old_nickname"}, true, nil)
				d.editors.EXPECT().Rename(mock.Anything, editor.EditorID, editor.Nickname).Return(errStore)
			},
			wantReason: errStore.Error(),
		},
		{
			name: "the edit itself cannot be fetched",
			arrange: func(d *deps) {
				d.editorIsKnown()
				d.wiki.EXPECT().FetchEdit(mock.Anything, mock.Anything, mock.Anything, locale).
					Return(entity.Edit{}, errWiki)
			},
			wantReason: errWiki.Error(),
		},
		{
			name: "the priced row cannot be written",
			arrange: func(d *deps) {
				d.editorIsKnown()
				d.editIsFetchable()
				d.pricesAt(100)
				d.revisions.EXPECT().Upsert(mock.Anything, mock.Anything).Return(errStore)
			},
			wantReason: errStore.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edit := change(11, "(AE) article edit")

			d := newDeps(t)
			d.storedAt(entity.SyncState{})
			saved := d.saves()
			d.wikiReturns([]entity.RecentChange{edit})
			parked := d.deadLetters()
			tt.arrange(d)

			require.NoError(t, d.service().Sync(context.Background()),
				"one revision failing is not the locale failing")

			failed := parked.last(t)
			assert.Equal(t, edit.RevID, failed.RevID)
			assert.Equal(t, locale, failed.Locale)
			assert.Equal(t, entity.ArticleEdit, failed.Type)
			assert.Equal(t, entity.FailedPending, failed.Status)
			assert.Equal(t, 1, failed.Attempts)
			assert.Contains(t, failed.LastError, tt.wantReason)
			assert.Equal(t, edit.UserID, failed.WikiUserID, "the raw wiki identity is all there is to go on")
			assert.Equal(t, edit.User, failed.Nickname)

			assert.Equal(t, edit.RevID, saved.last(t).LastRevID,
				"the cursor moves past a parked revision, or one broken page wedges the locale forever")
		})
	}
}

func TestService_KeepsTheCursorWhenTheWikiIsDown(t *testing.T) {
	d := newDeps(t)
	d.storedAt(entity.SyncState{LastRevID: 9, LastEditedAt: firstEdit})
	d.wiki.EXPECT().FetchRecentChanges(mock.Anything, locale, mock.Anything, d.cfg.BatchSize).
		Return(nil, errWiki)

	// No Save is set up: writing a cursor here would skip whatever the failed
	// call was going to return.
	err := d.service().Sync(context.Background())

	require.ErrorIs(t, err, errWiki)
	assert.ErrorContains(t, err, locale, "a joined error has to say which locale gave way")
}

func TestService_LeavesRecentlySyncedLocalesAlone(t *testing.T) {
	tests := []struct {
		name      string
		updatedAt time.Time
		wantFetch bool
	}{
		{"synced a moment ago", time.Now(), false},
		{"synced long ago", time.Now().Add(-time.Hour), true},
		{"never synced", time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			d.cfg.MinInterval = time.Minute
			d.storedAt(entity.SyncState{LastEditedAt: firstEdit, UpdatedAt: tt.updatedAt})

			// A throttled locale sets up neither the wiki nor Save, so touching
			// either fails the test.
			if tt.wantFetch {
				d.saves()
				d.wikiReturns(nil)
			}

			require.NoError(t, d.service().Sync(context.Background()))
		})
	}
}

func TestService_SkipsWhatIsAlreadyBehindTheCursor(t *testing.T) {
	d := newDeps(t)
	d.cfg.BatchSize = 3
	d.storedAt(entity.SyncState{LastRevID: 5, LastEditedAt: firstEdit})
	saved := d.saves()
	// rcstart is a timestamp and inclusive, so a batch opens with edits that
	// were handled on the previous pass.
	d.wikiReturns([]entity.RecentChange{
		change(4, "(AE) old"),
		change(5, "(AE) already counted"),
		change(6, "(AE) new"),
	})
	d.editorIsKnown()
	d.editIsFetchable()
	d.pricesAt(1000)
	stored := d.stores()

	require.NoError(t, d.service().Sync(context.Background()))

	require.Len(t, stored.all(), 1, "only the edit past the cursor is priced")
	assert.Equal(t, int64(6), stored.last(t).RevID)
	assert.Equal(t, int64(6), saved.last(t).LastRevID)
}

func TestService_StopsWhenABatchBringsNothingNew(t *testing.T) {
	d := newDeps(t)
	d.storedAt(entity.SyncState{LastRevID: 9, LastEditedAt: firstEdit})
	d.saves()
	// A full batch, every edit of it already seen. Asking again would return
	// the same batch, which is how a cursor that cannot advance spins forever.
	fetches := d.wikiReturns([]entity.RecentChange{
		change(8, "(AE) old"),
		change(9, "(AE) old"),
	})

	require.NoError(t, d.service().Sync(context.Background()))

	assert.Equal(t, 1, fetches.len(), "a batch with nothing new ends the pass")
}

func TestService_PagesFromTheLastEditItRead(t *testing.T) {
	d := newDeps(t)
	d.storedAt(entity.SyncState{})
	d.saves()
	fetches := d.wikiReturns(
		[]entity.RecentChange{change(11, "(ME) one"), change(12, "(ME) two")},
		[]entity.RecentChange{change(13, "(ME) three")},
	)
	d.editorIsKnown()
	d.editIsFetchable()
	d.pricesAt(1500)
	stored := d.stores()

	require.NoError(t, d.service().Sync(context.Background()))

	require.Len(t, fetches.all(), 2, "a full batch means there may be more behind it")
	assert.Equal(t, change(12, "").Timestamp, fetches.all()[1],
		"the second page starts where the first one ended")
	assert.Len(t, stored.all(), 3)
}

func TestService_KeepsLocalesApart(t *testing.T) {
	const other = "en"

	d := newDeps(t)
	d.cfg.Locales = []string{locale, other}

	d.storedAt(entity.SyncState{LastEditedAt: firstEdit})
	d.wiki.EXPECT().FetchRecentChanges(mock.Anything, locale, mock.Anything, d.cfg.BatchSize).
		Return(nil, errWiki)

	d.state.EXPECT().Get(mock.Anything, other).Return(entity.SyncState{LastEditedAt: firstEdit}, nil)
	d.wiki.EXPECT().FetchRecentChanges(mock.Anything, other, mock.Anything, d.cfg.BatchSize).
		Return(nil, nil)
	saved := d.saves()

	err := d.service().Sync(context.Background())

	require.ErrorIs(t, err, errWiki)
	require.Len(t, saved.all(), 1, "the healthy locale finished its pass anyway")
	assert.Equal(t, other, saved.last(t).Locale)
}

func TestService_Locking(t *testing.T) {
	t.Run("a locale somebody else is already on is left to them", func(t *testing.T) {
		d := newDeps(t)
		locker := mocks.NewMockLocker(t)
		locker.EXPECT().TryLock(mock.Anything, mock.Anything).Return(false, nil, nil)
		d.locker = locker

		// Neither the state nor the wiki is set up: a locale we did not claim
		// must not be touched at all.
		require.NoError(t, d.service().Sync(context.Background()))
	})

	t.Run("the lock is handed back when the pass is over", func(t *testing.T) {
		d := newDeps(t)
		var released atomic.Bool

		locker := mocks.NewMockLocker(t)
		locker.EXPECT().TryLock(mock.Anything, mock.Anything).
			Return(true, func() { released.Store(true) }, nil)
		d.locker = locker

		d.storedAt(entity.SyncState{LastEditedAt: firstEdit})
		d.wikiReturns(nil)
		d.saves()

		require.NoError(t, d.service().Sync(context.Background()))
		assert.True(t, released.Load(), "a lock held past the pass blocks every later sync")
	})

	t.Run("a lock that cannot be taken is a failure, not a skip", func(t *testing.T) {
		d := newDeps(t)
		locker := mocks.NewMockLocker(t)
		locker.EXPECT().TryLock(mock.Anything, mock.Anything).Return(false, nil, errStore)
		d.locker = locker

		require.ErrorIs(t, d.service().Sync(context.Background()), errStore)
	})
}

func TestService_CollapsesConcurrentCalls(t *testing.T) {
	const callers = 50

	d := newDeps(t)
	d.storedAt(entity.SyncState{LastEditedAt: firstEdit})
	d.saves()

	var runs atomic.Int64
	d.wiki.EXPECT().FetchRecentChanges(mock.Anything, locale, mock.Anything, d.cfg.BatchSize).
		RunAndReturn(func(context.Context, string, time.Time, int) ([]entity.RecentChange, error) {
			runs.Add(1)
			// Holding the run open widens the window the others arrive in. It
			// only ever makes this test more certain: without the collapse
			// every caller runs regardless of how long each one takes.
			time.Sleep(50 * time.Millisecond)

			return nil, nil
		})

	svc := d.service()
	start := make(chan struct{})

	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			assert.NoError(t, svc.Sync(context.Background()))
		}()
	}

	close(start)
	wg.Wait()

	assert.Less(t, runs.Load(), int64(callers),
		"callers that turn up during a run share it instead of starting their own")
}
