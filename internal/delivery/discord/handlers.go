package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/the-new-day/protanki-wiki-admin/internal/storage"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/revisions"
)

const monthLayout = "2006-01"

// commandFunc does the actual work for a command, off the 3-second interaction
// ack deadline. Its result is JSON-encoded into the edited reply.
type commandFunc func(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) (any, error)

func (b *Bot) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()

	switch data.Name {
	case "wheechair":
		b.replyText(s, i, "♿")
	case "salary":
		b.runGated(s, i, data, []string{b.wikiRoleID, b.wikiAdminRoleID}, false, b.handleSalary)
	case "edits":
		b.runGated(s, i, data, []string{b.wikiRoleID, b.wikiAdminRoleID}, false, b.handleEdits)
	case "report":
		b.runGated(s, i, data, []string{b.wikiAdminRoleID}, false, b.handleReport)
	case "changepay":
		// Its result can carry a locale-required nudge meant for the caller
		// only, so the whole exchange stays ephemeral.
		b.runGated(s, i, data, []string{b.wikiAdminRoleID}, true, b.handleChangePay)
	}
}

// runGated checks role membership, then immediately defers so Discord stops waiting
// on the 3-second ack window, and runs fn in the background. When fn returns,
// the deferred "thinking" message is edited into the real result.
func (b *Bot) runGated(
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
	roleIDs []string,
	ephemeral bool,
	fn commandFunc,
) {
	if i.Member == nil || !hasAnyRole(i.Member, roleIDs...) {
		b.replyTextEphemeral(s, i, "You are not allowed to run this command.")
		return
	}

	if !b.deferReply(s, i, ephemeral) {
		return
	}

	go func() {
		result, err := fn(context.Background(), i, data)
		if err != nil {
			b.editReplyError(s, i, err)
			return
		}

		b.editReplyJSON(s, i, result)
	}()
}

func (b *Bot) handleSalary(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
) (any, error) {
	nickname := data.GetOption("nickname").StringValue()

	from, to, err := monthRange(optionalString(data, "month"))
	if err != nil {
		return nil, err
	}

	payslip, err := b.earnings.ForNickname(ctx, nickname, from, to)
	if err != nil {
		return nil, err
	}

	return struct {
		Editor any       `json:"editor"`
		From   time.Time `json:"from"`
		To     time.Time `json:"to"`
		Total  int64     `json:"total"`
	}{payslip.Editor, payslip.From, payslip.To, payslip.Total}, nil
}

func (b *Bot) handleEdits(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
) (any, error) {
	nickname := data.GetOption("nickname").StringValue()

	from, to, err := monthRange(optionalString(data, "month"))
	if err != nil {
		return nil, err
	}

	return b.earnings.ForNickname(ctx, nickname, from, to)
}

func (b *Bot) handleReport(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
) (any, error) {
	from, to, err := monthRange(optionalString(data, "month"))
	if err != nil {
		return nil, err
	}

	return b.earnings.Report(ctx, from, to)
}

func (b *Bot) handleChangePay(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
) (any, error) {
	nickname := data.GetOption("nickname").StringValue()
	editID := data.GetOption("edit_id").IntValue()
	newCost := data.GetOption("new_cost").IntValue()
	locale := optionalString(data, "locale")
	changedBy := fmt.Sprintf("%s (%s)", i.Member.User.Username, i.Member.User.ID)

	return b.revisions.OverridePrice(ctx, nickname, locale, editID, newCost, changedBy)
}

func hasAnyRole(member *discordgo.Member, roleIDs ...string) bool {
	for _, roleID := range roleIDs {
		if roleID != "" && slices.Contains(member.Roles, roleID) {
			return true
		}
	}

	return false
}

// monthRange parses "YYYY-MM" into a half-open [from, to) period covering
// that month. An empty raw defaults to the current month.
func monthRange(raw string) (time.Time, time.Time, error) {
	var from time.Time

	if raw == "" {
		now := time.Now().UTC()
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	} else {
		parsed, err := time.Parse(monthLayout, raw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("discord: month %q: YYYY-MM format is expected: %w", raw, err)
		}
		from = time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	return from, from.AddDate(0, 1, 0), nil
}

func optionalString(data discordgo.ApplicationCommandInteractionData, name string) string {
	opt := data.GetOption(name)
	if opt == nil {
		return ""
	}

	return opt.StringValue()
}

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
		log.Printf("discord: edit reply: %v", err)
	}
}

// editReplyJSON dumps v as a raw JSON code block. This is a placeholder
// response format -- formatted embeds can come later.
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
		b.editReplyText(s, i, err.Error())
	}
}
