package discord

import (
	"errors"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/the-new-day/protanki-wiki-admin/internal/storage"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/revisions"
)

func (b *Bot) replyTextEphemeral(i *discordgo.InteractionCreate, content string) {
	err := b.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("discord: reply ephemeral: %v", err)
	}
}

// deferReply acknowledges the interaction immediately with a "thinking..."
// placeholder, buying up to 15 minutes to edit in the real response instead
// of the default 3 seconds.
func (b *Bot) deferReply(i *discordgo.InteractionCreate, ephemeral bool) bool {
	resp := &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredChannelMessageWithSource}
	if ephemeral {
		resp.Data = &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral}
	}

	if err := b.session.InteractionRespond(i.Interaction, resp); err != nil {
		log.Printf("discord: defer: %v", err)
		return false
	}

	return true
}

func (b *Bot) editReplyText(i *discordgo.InteractionCreate, content string) string {
	msg, err := b.session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &content})
	if err != nil {
		b.editReplyError(i, err)
	}
	if msg != nil {
		return msg.ID
	}
	return ""
}

func (b *Bot) editReplyError(i *discordgo.InteractionCreate, err error) {
	switch {
	case errors.Is(err, revisions.ErrLocaleRequired):
		b.replyTextEphemeral(i, fmt.Sprintf("%v. This editor has multiple accounts. Specify the locale and repeat the command.", err))
	case errors.Is(err, storage.ErrNotFound):
		b.replyTextEphemeral(i, "The editor or the edit not found.")
	case errors.Is(err, ErrWrongMonthLayout):
		b.replyTextEphemeral(i, "Wrong month layout. Use YYYY-MM, for example 2026-08.")
	default:
		log.Printf("discord: edit reply: %v", err)
		b.replyTextEphemeral(i, "Something went wrong. Please let the nearest nerd know.")
	}
}

func (b *Bot) removeReply(i *discordgo.InteractionCreate, messageID string) {
	b.session.ChannelMessageDelete(i.ChannelID, messageID)
}
