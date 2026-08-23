package discord

import (
	"errors"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/the-new-day/wiki-earnings/internal/storage"
	"github.com/the-new-day/wiki-earnings/internal/usecase/revisions"
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
func (b *Bot) deferReply(i *discordgo.InteractionCreate, ephemeral bool) (ok bool) {
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

// errorText maps err to something worth putting in front of the user.
func errorText(err error) (text string, hasExplanation bool) {
	switch {
	case errors.Is(err, revisions.ErrLocaleRequired):
		return fmt.Sprintf("%v. This editor has multiple accounts. Specify the locale and repeat the command.", err), true
	case errors.Is(err, storage.ErrNotFound):
		return "The editor or the edit not found.", true
	case errors.Is(err, ErrNoTaskChannels):
		return "No task channels are configured. Configure and restart the bot.", true
	case errors.Is(err, ErrWrongMonthLayout):
		return "Wrong month layout. Use YYYY-MM, for example 2026-08.", true
	default:
		return "Something went wrong. Please let the nearest nerd know.", false
	}
}

// reply is the set of messages one interaction's answer is spread across: the deferred response,
// plus a followup for every extra piece Discord's lengthcap forces.
type reply struct {
	bot         *Bot
	interaction *discordgo.InteractionCreate
	ephemeral   bool

	rootID      string
	followupIDs []string
}

func (b *Bot) newReply(i *discordgo.InteractionCreate, ephemeral bool) *reply {
	return &reply{bot: b, interaction: i, ephemeral: ephemeral}
}

// setText replaces whatever the reply currently shows with content, splitting
// it across as many messages as it takes.
func (r *reply) setText(content string) {
	chunks := splitMessage(content, maxMessageLength)

	msg, err := r.bot.session.InteractionResponseEdit(
		r.interaction.Interaction,
		&discordgo.WebhookEdit{Content: &chunks[0]},
	)
	if err != nil {
		r.fail(err)
		return
	}
	if msg != nil {
		r.rootID = msg.ID
	}

	rest := chunks[1:]
	r.writeFollowups(rest)
	r.trimFollowups(len(rest))
}

// writeFollowups edits the followups already posted and creates the ones still missing.
func (r *reply) writeFollowups(chunks []string) {
	for idx, chunk := range chunks {
		if idx < len(r.followupIDs) {
			_, err := r.bot.session.FollowupMessageEdit(
				r.interaction.Interaction,
				r.followupIDs[idx],
				&discordgo.WebhookEdit{Content: &chunk},
			)
			if err != nil {
				log.Printf("discord: edit followup: %v", err)
				return
			}

			continue
		}

		msg, err := r.bot.session.FollowupMessageCreate(r.interaction.Interaction, true, &discordgo.WebhookParams{
			Content: chunk,
			Flags:   r.flags(),
		})
		if err != nil {
			log.Printf("discord: create followup: %v", err)
			return
		}

		r.followupIDs = append(r.followupIDs, msg.ID)
	}
}

// trimFollowups deletes the followups past keep, which the text the reply now
// shows is too short to fill.
func (r *reply) trimFollowups(keep int) {
	if keep >= len(r.followupIDs) {
		return
	}

	for _, id := range r.followupIDs[keep:] {
		if err := r.bot.session.FollowupMessageDelete(r.interaction.Interaction, id); err != nil {
			log.Printf("discord: delete followup: %v", err)
		}
	}

	r.followupIDs = r.followupIDs[:keep]
}

// remove deletes every message the reply is made of, including the
// "thinking..." placeholder if nothing has replaced it yet.
func (r *reply) remove() {
	r.trimFollowups(0)

	if r.ephemeral || r.rootID == "" {
		if err := r.bot.session.InteractionResponseDelete(r.interaction.Interaction); err != nil {
			log.Printf("discord: delete reply: %v", err)
		}

		return
	}

	if err := r.bot.session.ChannelMessageDelete(r.interaction.ChannelID, r.rootID); err != nil {
		log.Printf("discord: delete reply: %v", err)
	}
}

// fail tells the user privately what went wrong and takes the reply down,
// placeholder included.
func (r *reply) fail(err error) {
	text, known := errorText(err)
	if !known {
		log.Printf("discord: %v", err)
	}

	_, sendErr := r.bot.session.FollowupMessageCreate(r.interaction.Interaction, true, &discordgo.WebhookParams{
		Content: text,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
	if sendErr != nil {
		log.Printf("discord: send error: %v", sendErr)
	}

	r.remove()
}

func (r *reply) flags() discordgo.MessageFlags {
	if r.ephemeral {
		return discordgo.MessageFlagsEphemeral
	}

	return 0
}
