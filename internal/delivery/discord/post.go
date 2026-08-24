package discord

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
)

var ErrNoTaskChannels = errors.New("discord: no task channels configured")

// PostTask sends each locale's text to that locale's channel. A channel that
// rejects the message does not stop the others, and the failures come back
// joined so the caller can say which.
//
// Locales are taken in sorted order, so a run posts in the same order every time.
func (b *Bot) PostTask(_ context.Context, localizedTexts map[string]string) error {
	if len(b.taskChannels) == 0 {
		return ErrNoTaskChannels
	}

	var errs []error

	for _, locale := range slices.Sorted(maps.Keys(localizedTexts)) {
		channelID, ok := b.taskChannels[locale]
		if !ok {
			errs = append(errs, fmt.Errorf("locale %s has no channel", locale))
			continue
		}

		if err := b.postChunked(channelID, localizedTexts[locale]); err != nil {
			errs = append(errs, fmt.Errorf("%s channel %s: %w", locale, channelID, err))
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
