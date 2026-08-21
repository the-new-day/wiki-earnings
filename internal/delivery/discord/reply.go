package discord

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/the-new-day/protanki-wiki-admin/internal/storage"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/revisions"
)

func (b *Bot) replyText(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	})
	if err != nil {
		log.Printf("discord: reply: %v", err)
	}
}

func (b *Bot) replyTextEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
func (b *Bot) deferReply(s *discordgo.Session, i *discordgo.InteractionCreate, ephemeral bool) bool {
	resp := &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredChannelMessageWithSource}
	if ephemeral {
		resp.Data = &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral}
	}

	if err := s.InteractionRespond(i.Interaction, resp); err != nil {
		log.Printf("discord: defer: %v", err)
		return false
	}

	return true
}

func (b *Bot) editReplyText(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &content})
	if err != nil {
		b.editReplyError(s, i, err)
	}
}

// editReplyJSON dumps v as a raw JSON code block.
func (b *Bot) editReplyJSON(s *discordgo.Session, i *discordgo.InteractionCreate, v any) {
	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		b.editReplyError(s, i, err)
		return
	}

	b.editReplyText(s, i, fmt.Sprintf("```json\n%s\n```", encoded))
}

func (b *Bot) editReplyError(s *discordgo.Session, i *discordgo.InteractionCreate, err error) {
	switch {
	case errors.Is(err, revisions.ErrLocaleRequired):
		b.editReplyText(s, i, fmt.Sprintf("%v. Specify the locale and repeat the command.", err))
	case errors.Is(err, storage.ErrNotFound):
		b.editReplyText(s, i, "The editor or the edit not found.")
	default:
		log.Printf("discord: edit reply: %v", err)
		b.editReplyText(s, i, "Something went wrong. Please let the nearest nerd know.")
	}
}
