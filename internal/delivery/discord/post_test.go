package discord

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPostTask(t *testing.T) {
	tests := []struct {
		name            string
		channels        map[string]string
		localizedTexts  map[string]string
		wantErr         error
		wantErrMentions []string
	}{
		{
			// Nothing configured has to be an error rather than a quiet
			// success: the command would otherwise report a task nobody
			// received.
			name:           "nothing configured is refused",
			channels:       nil,
			localizedTexts: map[string]string{"ru": "text"},
			wantErr:        ErrNoTaskChannels,
		},
		{
			name:            "a locale with no channel is reported",
			channels:        map[string]string{"ru": "100"},
			localizedTexts:  map[string]string{"ua": "text"},
			wantErrMentions: []string{"ua"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bot := &Bot{taskChannels: tt.channels}

			err := bot.PostTask(context.Background(), tt.localizedTexts)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			}
			for _, mention := range tt.wantErrMentions {
				assert.ErrorContains(t, err, mention)
			}
		})
	}
}
