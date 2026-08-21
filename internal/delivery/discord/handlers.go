package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/pricing"
	"github.com/the-new-day/protanki-wiki-admin/internal/mediawiki"
	"github.com/the-new-day/protanki-wiki-admin/internal/storage"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/revisions"
)

const monthLayout = "2006-01"
const dayLayout = "2006-01-02"

// commandFunc does the actual work for a command, off the 3-second interaction
// ack deadline. Its result is pasted into the edited reply.
type commandFunc func(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData) (string, error)

func (b *Bot) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()

	switch data.Name {
	case "salary":
		b.runGated(s, i, data, []string{b.wikiRoleID, b.wikiAdminRoleID}, true, b.handleSalary)
	case "edits":
		b.runGated(s, i, data, []string{b.wikiRoleID, b.wikiAdminRoleID}, true, b.handleEdits)
	case "report":
		b.runGated(s, i, data, []string{b.wikiAdminRoleID}, false, b.handleReport)
	case "changepay":
		// Its result can carry a locale-required nudge meant for the caller
		// only, so the whole exchange stays ephemeral.
		b.runGated(s, i, data, []string{b.wikiAdminRoleID}, true, b.handleChangePay)
	case "commands":
		b.runGated(s, i, data, []string{b.wikiAdminRoleID}, false, b.handleCommands)
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
		b.replyTextEphemeral(s, i, "You are not allowed to run this command. ♿")
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

		b.editReplyText(s, i, result)
	}()
}

func (b *Bot) handleSalary(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
) (string, error) {
	nickname := data.GetOption("nickname").StringValue()

	from, to, err := monthRange(optionalString(data, "month"))
	if err != nil {
		return "", err
	}

	payslip, err := b.earnings.ForNickname(ctx, nickname, from, to)
	if err != nil {
		return "", err
	}

	result := fmt.Sprintf(
		"Wiki Editor: %s\nPeriod: %s\nEarned: %s",
		nickname, monthToText(from), earningsToString(int(payslip.Total)),
	)

	if payslip.SyncErr != nil {
		log.Printf("sync err: %s", payslip.SyncErr)
		return fmt.Sprintf("%s\n\n:warning: Wiki Sync error, results may be out of date.", result), nil
	}

	return result, nil
}

func (b *Bot) handleEdits(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
) (string, error) {
	nickname := data.GetOption("nickname").StringValue()
	showMinorEdits := optionalBool(data, "show_minor")

	from, to, err := monthRange(optionalString(data, "month"))
	if err != nil {
		return "", err
	}

	payslip, err := b.earnings.ForNickname(ctx, nickname, from, to)
	if err != nil {
		return "", err
	}

	var body strings.Builder

	for _, rev := range payslip.Revisions {
		if !rev.CostOverridden && !showMinorEdits && rev.Type.IsMinor() {
			continue
		}

		// example:
		// * [2026-08-20] ((NA)): [Main Page](<https://wiki.pro-tanki.online/en/Main_Page>)
		//   10 310 💎 (changed)
		costChanged := ""
		if rev.CostOverridden {
			costChanged = "changed"
		}

		fmt.Fprintf(&body,
			"* [%s] %s: [%s](<%s%s/%s>)\n  %d 💎 %s\n",
			rev.EditedAt.Format(dayLayout), revTypeToString(rev.Type),
			rev.PageTitle, mediawiki.WikiUrl, rev.Locale, rev.PageTitle, rev.Cost, costChanged,
		)
	}

	var result strings.Builder
	fmt.Fprintf(&result, "Wiki Editor: %s\n", nickname)
	fmt.Fprintf(&result, "Period: %s\n", monthToText(from))
	fmt.Fprintf(&result, "Total edits: %d\n\n", len(payslip.Revisions))
	fmt.Fprint(&result, body.String())

	if payslip.SyncErr != nil {
		log.Printf("sync err: %s", payslip.SyncErr)
		return fmt.Sprintf("%s\n\n:warning: Wiki Sync error, results may be out of date.", result.String()), nil
	}

	return result.String(), nil
}

func (b *Bot) handleReport(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
) (string, error) {
	from, to, err := monthRange(optionalString(data, "month"))
	if err != nil {
		return "", err
	}

	report, err := b.earnings.Report(ctx, from, to)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	fmt.Fprint(&result, "### Earnings report\n")
	fmt.Fprintf(&result, "Period: %s\n", monthToText(from))
	fmt.Fprintf(&result, "Total crystals earned: %d\n\n", report.Total)

	for _, editorEarnings := range report.Editors {
		if editorEarnings.Total == 0 {
			continue
		}

		fmt.Fprintf(&result,
			"* **%s**: %s\n",
			editorEarnings.Nickname, earningsToString(int(editorEarnings.Total)),
		)
	}

	if report.SyncErr != nil {
		log.Printf("sync err: %s", report.SyncErr)
		return fmt.Sprintf("%s\n\n:warning: Wiki Sync error, results may be out of date.", result.String()), nil
	}

	return result.String(), nil
}

func (b *Bot) handleChangePay(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
) (string, error) {
	nickname := data.GetOption("nickname").StringValue()
	editID := data.GetOption("edit_id").IntValue()
	newCost := data.GetOption("new_cost").IntValue()
	locale := optionalString(data, "locale")
	changedBy := fmt.Sprintf("%s (%s)", i.Member.User.Username, i.Member.User.ID)

	_, err := b.revisions.OverridePrice(ctx, nickname, locale, editID, newCost, changedBy)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Cost changed to %d.", newCost), nil
}

func (b *Bot) handleCommands(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
) (string, error) {
	from, to, err := monthRange(optionalString(data, "month"))
	if err != nil {
		return "", err
	}

	report, err := b.earnings.Report(ctx, from, to)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	result.WriteString("```\n")

	for _, editorEarnings := range report.Editors {
		if editorEarnings.Total == 0 {
			continue
		}

		fmt.Fprintf(&result,
			"/givecry %s %d\n",
			editorEarnings.Nickname,
			editorEarnings.Total,
		)
	}

	result.WriteByte('\n')

	for _, editorEarnings := range report.Editors {
		if editorEarnings.Total < pricing.CrystalsForPremDay {
			continue
		}

		fmt.Fprintf(&result,
			"/addpremium %s %d\n",
			editorEarnings.Nickname,
			pricing.DaysPremium(int(editorEarnings.Total)),
		)
	}

	result.WriteString("```")
	return result.String(), nil
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

func optionalBool(data discordgo.ApplicationCommandInteractionData, name string) bool {
	opt := data.GetOption(name)
	if opt == nil {
		return false
	}

	return opt.BoolValue()
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
		log.Printf("discord: edit reply: %w", err)
		b.editReplyText(s, i, "Something went wrong. Please let the nearest nerd know.")
	}
}

func monthToText(timestamp time.Time) string {
	return timestamp.Format("January 2006")
}

func earningsToString(amount int) string {
	if amount < 0 {
		panic(fmt.Sprintf("earningsToString: amount = %d", amount))
	}

	daysOfPrem := pricing.DaysPremium(amount)

	if daysOfPrem == 0 {
		return fmt.Sprintf("%d 💎", amount)
	} else if daysOfPrem == 1 {
		return fmt.Sprintf("%d 💎 + 1 day of Premium", amount)
	}
	return fmt.Sprintf("%d 💎 + %d days of Premium", amount, daysOfPrem)
}

func revTypeToString(r entity.RevisionType) string {
	switch r {
	case entity.MinorEdit:
		return "((ME))"
	case entity.ItemAddition:
		return "((IA))"
	case entity.ArticleEdit:
		return "((AE))"
	case entity.RefactoredArticle:
		return "((RA))"
	case entity.NewArticle:
		return "((NA))"
	case entity.TranslatedArticle:
		return "((TA))"
	default:
		return "!!Unknown!!"
	}
}
