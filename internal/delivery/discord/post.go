package discord

import (
	"context"
	"errors"
	"fmt"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

var ErrNoTaskChannels = errors.New("discord: no task channels configured")

// PostTask sends each translation to the channels registered for its language.
// A channel that rejects the message does not stop the others, and the failures come back
// joined so the caller can say which.
func (b *Bot) PostTask(_ context.Context, localizedTexts map[entity.Language]string) error {
	if len(b.taskChannels) == 0 {
		return ErrNoTaskChannels
	}

	var errs []error

	for lang, text := range localizedTexts {
		for _, channelID := range b.taskChannels[lang] {
			if err := b.postChunked(channelID, text); err != nil {
				errs = append(errs, fmt.Errorf("%s channel %s: %w", lang, channelID, err))
			}
		}
	}

	return errors.Join(errs...)
}

// postChunked sends text as however many messages Discord's length cap needs,
// stopping at the first one that fails.
func (b *Bot) postChunked(channelID string, text string) error {
	for _, chunk := range splitMessage(text, maxMessageLength) {
		if err := b.sendTextMessage(channelID, chunk); err != nil {
			return err
		}
	}

	return nil
}
