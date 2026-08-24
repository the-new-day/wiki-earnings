package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
)

// Every message the bot sends suppresses embeds, whoever it is addressed to.
// A link that turns into a preview is never wanted, and the two answers used to
// disagree about it: /edits wrapped its links in angle brackets, /task did not.
func TestOutgoingFlags(t *testing.T) {
	tests := []struct {
		name      string
		ephemeral bool
		want      discordgo.MessageFlags
	}{
		{
			name:      "a public message",
			ephemeral: false,
			want:      discordgo.MessageFlagsSuppressEmbeds,
		},
		{
			name:      "a message only its caller sees",
			ephemeral: true,
			want:      discordgo.MessageFlagsSuppressEmbeds | discordgo.MessageFlagsEphemeral,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, outgoingFlags(tt.ephemeral))
		})
	}
}
