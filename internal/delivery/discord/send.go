package discord

import "github.com/bwmarrin/discordgo"

func outgoingFlags(ephemeral bool) discordgo.MessageFlags {
	flags := discordgo.MessageFlagsSuppressEmbeds
	if ephemeral {
		flags |= discordgo.MessageFlagsEphemeral
	}

	return flags
}

func (b *Bot) sendTextMessage(channelID string, message string) error {
	_, err := b.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: message,
		Flags:   outgoingFlags(false),
	})

	return err
}
