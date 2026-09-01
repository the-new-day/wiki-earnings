package corrections

import (
	"context"
	"fmt"
	"time"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

type Repository interface {
	AddCorrection(
		ctx context.Context,
		nickname string,
		description string,
		amount int64,
		createdBy string,
		appliesAt time.Time,
	) (entity.PaymentCorrection, error)

	RemoveCorrection(ctx context.Context, correctionID int64) error
}

type UseCase struct {
	correctionsRepo Repository
}

func New(correctionRepo Repository) *UseCase {
	return &UseCase{
		correctionsRepo: correctionRepo,
	}
}

// AddPaymentCorrection records a manual correction. appliesAt is what places it
// in a monthly report: pass the real time for "now", or the first instant of a
// past month to book it there. The row is always audited with its real insert
// time separately.
func (u *UseCase) AddPaymentCorrection(
	ctx context.Context,
	nickname string,
	description string,
	amount int64,
	createdBy string,
	appliesAt time.Time,
) (entity.PaymentCorrection, error) {
	correction, err := u.correctionsRepo.AddCorrection(ctx, nickname, description, amount, createdBy, appliesAt)
	if err != nil {
		return entity.PaymentCorrection{},
			fmt.Errorf("corrections: add correction for %s (%s) by %d: %w", nickname, description, amount, err)
	}

	return correction, nil
}

func (u *UseCase) RemoveCorrection(ctx context.Context, correctionID int64) error {
	if err := u.correctionsRepo.RemoveCorrection(ctx, correctionID); err != nil {
		return fmt.Errorf("corrections: remove correction %d: %w", correctionID, err)
	}

	return nil
}
