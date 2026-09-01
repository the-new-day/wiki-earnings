package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/storage"
	"github.com/the-new-day/wiki-earnings/internal/usecase/corrections"
	"github.com/the-new-day/wiki-earnings/internal/usecase/earnings"
)

var (
	_ corrections.Repository           = (*PaymentCorrectionRepository)(nil)
	_ earnings.PaymentCorrectionReader = (*PaymentCorrectionRepository)(nil)
)

type PaymentCorrectionRepository struct {
	pool *pgxpool.Pool
}

func NewPaymentCorrectionRepository(pool *pgxpool.Pool) *PaymentCorrectionRepository {
	return &PaymentCorrectionRepository{
		pool: pool,
	}
}

// SumByEditor totals corrections per editor over the half-open period
// [from, to), bucketed on applies_at (the month the correction is booked in),
// not created_at (audit). Editors with no correction in the period are not
// returned; totals may be negative. The editor's nickname and payments nickname
// are joined in so the report can name and pay the row without a second lookup.
func (repo *PaymentCorrectionRepository) SumByEditor(
	ctx context.Context,
	from, to time.Time,
) ([]entity.EditorEarnings, error) {
	rows, err := repo.pool.Query(ctx, `
		SELECT e.editor_id, e.nickname, COALESCE(e.payments_nickname, ''), COALESCE(SUM(c.correction_amount), 0)
		FROM editor_payment_corrections c
		JOIN editors e ON e.editor_id = c.editor_id
		WHERE c.applies_at >= $1 AND c.applies_at < $2
		GROUP BY e.editor_id, e.nickname, e.payments_nickname
		ORDER BY e.editor_id`, from, to)
	if err != nil {
		return nil, fmt.Errorf("postgres: sum corrections by editor: %w", err)
	}
	defer rows.Close()

	var out []entity.EditorEarnings
	for rows.Next() {
		var e entity.EditorEarnings
		if err := rows.Scan(&e.EditorID, &e.Nickname, &e.PaymentsNickname, &e.Total); err != nil {
			return nil, fmt.Errorf("postgres: sum corrections by editor: scan: %w", err)
		}
		out = append(out, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: sum corrections by editor: %w", err)
	}

	return out, nil
}

func (repo *PaymentCorrectionRepository) ListByEditor(
	ctx context.Context,
	editorID int64,
	from, to time.Time,
) ([]entity.PaymentCorrection, error) {
	rows, err := repo.pool.Query(ctx, `
		SELECT correction_id, editor_id, correction_amount, description, created_by, created_at, applies_at
		FROM editor_payment_corrections
		WHERE editor_id = $1 AND applies_at >= $2 AND applies_at < $3
		ORDER BY applies_at, created_at`, editorID, from, to)
	if err != nil {
		return nil, fmt.Errorf("postgres: list corrections for editor %d: %w", editorID, err)
	}
	defer rows.Close()

	var out []entity.PaymentCorrection
	for rows.Next() {
		var corr entity.PaymentCorrection
		err := rows.Scan(
			&corr.CorrectionID,
			&corr.EditorID,
			&corr.Amount,
			&corr.Description,
			&corr.CreatedBy,
			&corr.CreatedAt,
			&corr.AppliesAt,
		)
		if err != nil {
			return nil, fmt.Errorf("postgres: list corrections for editor %d: scan: %w", editorID, err)
		}
		out = append(out, corr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list corrections for editor %d: %w", editorID, err)
	}

	return out, nil
}

func (repo *PaymentCorrectionRepository) AddCorrection(
	ctx context.Context,
	nickname string,
	description string,
	amount int64,
	createdBy string,
	appliesAt time.Time,
) (entity.PaymentCorrection, error) {
	var corr entity.PaymentCorrection

	err := repo.pool.QueryRow(ctx, `
		WITH editor AS (
			SELECT editor_id FROM editors WHERE nickname = $1
		)

		INSERT INTO editor_payment_corrections
		(editor_id, correction_amount, description, created_by, applies_at)
		SELECT editor_id, $2, $3, $4, $5 FROM editor
		WHERE editor_id IS NOT NULL
		RETURNING correction_id, editor_id, correction_amount, description, created_by, created_at, applies_at
		`, nickname, amount, description, createdBy, appliesAt).
		Scan(
			&corr.CorrectionID,
			&corr.EditorID,
			&corr.Amount,
			&corr.Description,
			&corr.CreatedBy,
			&corr.CreatedAt,
			&corr.AppliesAt,
		)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.PaymentCorrection{},
				fmt.Errorf("postgres: add correction for editor %q: %w", nickname, storage.ErrNotFound)
		}

		return entity.PaymentCorrection{},
			fmt.Errorf("postgres: add correction for editor %q: %w", nickname, err)
	}

	return corr, nil
}

func (repo *PaymentCorrectionRepository) RemoveCorrection(ctx context.Context, correctionID int64) error {
	cmdTag, err := repo.pool.Exec(ctx, `
		DELETE FROM editor_payment_corrections WHERE correction_id = $1
	`, correctionID)

	if err != nil {
		return fmt.Errorf("postgres: remove correction %d: %w", correctionID, err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: remove correction %d: %w", correctionID, storage.ErrNotFound)
	}

	return nil
}
