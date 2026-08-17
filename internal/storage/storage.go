package storage

import (
	"context"
	"time"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
)

type EditorRepository interface {
	GetByID(ctx context.Context, id int64) (entity.Editor, error)
	FindByNickname(ctx context.Context, nickname string) (entity.Editor, error)
	Create(ctx context.Context, e entity.Editor) (int64, error)
	UpdateNickname(ctx context.Context, id int64, nickname string) error
}

type EditorAccountRepository interface {
	FindByLocaleUser(ctx context.Context, locale string, userID int64) (entity.EditorAccount, error)
	Create(ctx context.Context, a entity.EditorAccount) error
}

type RevisionWriter interface {
	Upsert(ctx context.Context, r entity.Revision) error
}

type RevisionReader interface {
	SumCostByEditor(ctx context.Context, from, to time.Time) ([]entity.EditorEarnings, error)
	SumCostForEditor(ctx context.Context, editorID int64, from, to time.Time) (int64, error)
	ListByEditor(ctx context.Context, editorID int64, from, to time.Time) ([]entity.Revision, error)
}

type SyncStateRepository interface {
	Get(ctx context.Context, locale string) (entity.SyncState, error)
	Save(ctx context.Context, s entity.SyncState) error
}

type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
