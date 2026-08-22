package discord

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/the-new-day/protanki-wiki-admin/internal/storage"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/revisions"
)

func TestErrorText_ExplainsKnownFailures(t *testing.T) {
	for _, err := range []error{revisions.ErrLocaleRequired, storage.ErrNotFound, ErrWrongMonthLayout} {
		text, known := errorText(err)

		assert.True(t, known, "%v has no explanation", err)
		assert.NotEmpty(t, text)
	}
}

func TestErrorText_UnwrapsWrappedFailures(t *testing.T) {
	_, known := errorText(fmt.Errorf("read editor: %w", storage.ErrNotFound))

	assert.True(t, known)
}

func TestErrorText_HidesUnknownFailures(t *testing.T) {
	text, known := errorText(errors.New("pgx: connection refused"))

	assert.False(t, known)
	assert.NotContains(t, text, "pgx")
}
