package discord

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPostTask_RefusesWithoutChannels(t *testing.T) {
	bot := &Bot{}

	err := bot.PostTask(context.Background(), map[string]string{"ru": "text"})

	assert.ErrorIs(t, err, ErrNoTaskChannels)
}

func TestPostTask_ReportsLocalesWithoutAChannel(t *testing.T) {
	bot := &Bot{taskChannels: map[string]string{"ru": "100"}}

	err := bot.PostTask(context.Background(), map[string]string{"ua": "text"})

	assert.ErrorContains(t, err, "ua")
}
