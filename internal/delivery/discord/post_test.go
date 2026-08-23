package discord

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

// Nothing configured has to be an error rather than a quiet success: the
// command would otherwise report a task nobody received.
func TestPostTask_RefusesWithoutChannels(t *testing.T) {
	bot := &Bot{}

	err := bot.PostTask(context.Background(), map[entity.Language]string{entity.LangRU: "text"})

	assert.ErrorIs(t, err, ErrNoTaskChannels)
}
