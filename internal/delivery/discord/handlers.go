package discord

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/domain/pricing"
	"github.com/the-new-day/wiki-earnings/internal/mediawiki"
	"github.com/the-new-day/wiki-earnings/internal/usecase/earnings"
)

const monthLayout = "2006-01"
const dayLayout = "2006-01-02"

var ErrWrongMonthLayout = errors.New("wrong month layout")

// commandFunc does the actual work for a command, off the 3-second interaction
// ack deadline. Its result is pasted into the edited reply. update lets it
// push an interim edit before returning - e.g. cached data shown instantly,
// replaced once a slower refresh finishes.
type commandFunc func(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
	initialText func(text string),
) (string, error)

func (b *Bot) handleInteractionCreate(
	i *discordgo.InteractionCreate,
	messageLifetime time.Duration,
) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleCommand(i, messageLifetime)
	case discordgo.InteractionApplicationCommandAutocomplete:
		b.handleAutocomplete(i)
	}
}

func (b *Bot) handleCommand(
	i *discordgo.InteractionCreate,
	messageLifetime time.Duration,
) {
	data := i.ApplicationCommandData()

	switch data.Name {
	case "salary":
		b.runGated(i, data, []string{b.wikiRoleID, b.wikiAdminRoleID}, true, b.handleSalary, 0)
	case "edits":
		b.runGated(i, data, []string{b.wikiRoleID, b.wikiAdminRoleID}, false, b.handleEdits, messageLifetime)
	case "task":
		b.runGated(i, data, []string{b.wikiAdminRoleID}, true, b.handleTask, 0)
	case "report":
		b.runGated(i, data, []string{b.wikiAdminRoleID}, false, b.handleReport, 0)
	case "changepay":
		b.runGated(i, data, []string{b.wikiAdminRoleID}, true, b.handleChangePay, 0)
	case "commands":
		b.runGated(i, data, []string{b.wikiAdminRoleID}, false, b.handleCommands, 0)
	case "resync":
		b.runGated(i, data, []string{b.wikiAdminRoleID}, true, b.handleResync, 0)
	}
}

func (b *Bot) handleAutocomplete(i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	focused := focusedOption(data)
	if focused == nil || data.Name != "task" || focused.Name != "locales" {
		return
	}

	raw, _ := focused.Value.(string)

	err := b.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: localeSuggestions(raw, b.tasks.Locales()),
		},
	})
	if err != nil {
		log.Printf("discord: autocomplete: %v", err)
	}
}

func focusedOption(
	data discordgo.ApplicationCommandInteractionData,
) *discordgo.ApplicationCommandInteractionDataOption {
	for _, opt := range data.Options {
		if opt.Focused {
			return opt
		}
	}

	return nil
}

// runGated checks role membership, then immediately defers so Discord stops waiting
// on the 3-second ack window, and runs fn in the background. When fn returns,
// the deferred "thinking" message is edited into the real result.
// If messageLifetime is not zero, the message will be deleted after that time passes.
func (b *Bot) runGated(
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
	roleIDs []string,
	ephemeral bool,
	fn commandFunc,
	messageLifetime time.Duration,
) {
	if i.Member == nil || !hasAnyRole(i.Member, roleIDs...) {
		b.replyTextEphemeral(i, "You are not allowed to run this command. ♿")
		return
	}

	if !b.deferReply(i, ephemeral) {
		return
	}

	rep := b.newReply(i, ephemeral)

	go func() {
		result, err := fn(context.Background(), i, data, rep.setText)
		if err != nil {
			rep.fail(err)
			return
		}

		rep.setText(result)

		if messageLifetime > 0 {
			time.AfterFunc(messageLifetime, rep.remove)
		}
	}()
}

func (b *Bot) handleSalary(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
	initialText func(string),
) (string, error) {
	nickname := data.GetOption("nickname").StringValue()

	from, to, err := monthRange(optionalString(data, "month"))
	if err != nil {
		return "", err
	}

	if cached, err := b.earnings.ReadForNickname(ctx, nickname, from, to); err == nil {
		initialText(formatSalary(nickname, from, cached.Total) + "\n-# Refreshing with latest edits...")
	}

	payslip, err := b.earnings.ForNickname(ctx, nickname, from, to)
	if err != nil {
		return "", err
	}

	result := formatSalary(nickname, from, payslip.Total)

	if payslip.SyncErr != nil {
		log.Printf("sync err: %s", payslip.SyncErr)
		return fmt.Sprintf("%s\n:warning: Results may be out of date.", result), nil
	}

	return result, nil
}

func formatSalary(nickname string, from time.Time, total int64) string {
	return fmt.Sprintf(
		"Wiki Editor: %s\nPeriod: %s\nEarned: %s",
		nickname, monthToText(from), earningsToString(int(total)),
	)
}

func (b *Bot) handleEdits(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
	initialText func(string),
) (string, error) {
	nickname := data.GetOption("nickname").StringValue()
	showMinorEdits := optionalBool(data, "show_minor")

	from, to, err := monthRange(optionalString(data, "month"))
	if err != nil {
		return "", err
	}

	if cached, err := b.earnings.ReadForNickname(ctx, nickname, from, to); err == nil {
		initialText(formatEdits(nickname, from, cached, showMinorEdits) + "\n-# Refreshing with latest edits...")
	}

	payslip, err := b.earnings.ForNickname(ctx, nickname, from, to)
	if err != nil {
		return "", err
	}

	result := formatEdits(nickname, from, payslip, showMinorEdits)

	if payslip.SyncErr != nil {
		log.Printf("sync err: %s", payslip.SyncErr)
		return fmt.Sprintf("%s\n:warning: Results may be out of date.", result), nil
	}

	return result, nil
}

func formatEdits(nickname string, from time.Time, payslip earnings.Payslip, showMinorEdits bool) string {
	var body strings.Builder

	autoMinorEditsCount := map[entity.RevisionType]int{}

	for _, rev := range payslip.Revisions {
		if !rev.CostOverridden && !showMinorEdits && rev.Type.IsMinor() {
			autoMinorEditsCount[rev.Type]++
			continue
		}

		// example:
		// * [2026-08-20|13052] ((NA)): [Main Page](<https://wiki.pro-tanki.online/en/Main_Page>)
		//   10 310 💎 (changed)
		costChangedText := ""
		if rev.CostOverridden {
			costChangedText = "(changed)"
		}

		fmt.Fprintf(&body,
			"* [%s|%d] %s: [%s](<%s%s/%s>)\n  %d 💎 %s\n",
			rev.EditedAt.Format(dayLayout), rev.RevID, revTypeToString(rev.Type),
			rev.PageTitle, mediawiki.WikiUrl, rev.Locale, strings.ReplaceAll(rev.PageTitle, " ", "_"),
			rev.Cost, costChangedText,
		)
	}

	var result strings.Builder
	fmt.Fprintf(&result, "Wiki Editor: %s\n", nickname)
	fmt.Fprintf(&result, "Period: %s\n", monthToText(from))
	fmt.Fprintf(&result, "Total edits: %d\n", len(payslip.Revisions))

	for revType, count := range autoMinorEditsCount {
		fmt.Fprintf(&result, "Auto %s count: %d\n", revTypeToString(revType), count)
	}

	result.WriteByte('\n')
	fmt.Fprint(&result, body.String())

	return result.String()
}

// handleTask posts a task to the locales it was addressed to, or to all of them
// when it was addressed to none, translated out of the language it was written
// in. That defaults to Russian.
func (b *Bot) handleTask(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
	_ func(string),
) (string, error) {
	text := data.GetOption("text").StringValue()
	locales := parseLocales(optionalString(data, "locales"))

	sourceLang, err := optionalLanguage(data, "source_lang", entity.LangRU)
	if err != nil {
		return "", err
	}

	if err := b.tasks.PostTask(ctx, text, sourceLang, locales); err != nil {
		return "", err
	}

	if len(locales) == 0 {
		return "Task posted to every locale.", nil
	}

	return fmt.Sprintf("Task posted to %s.", strings.Join(locales, ", ")), nil
}

func (b *Bot) handleReport(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
	initialText func(string),
) (string, error) {
	from, to, err := monthRange(optionalString(data, "month"))
	if err != nil {
		return "", err
	}

	if cached, err := b.earnings.ReadReport(ctx, from, to); err == nil {
		initialText(formatReport(from, cached) + "\n-# Refreshing with latest edits...")
	}

	report, err := b.earnings.Report(ctx, from, to)
	if err != nil {
		return "", err
	}

	result := formatReport(from, report)

	if report.SyncErr != nil {
		log.Printf("sync err: %s", report.SyncErr)
		return fmt.Sprintf("%s\n:warning: Results may be out of date.", result), nil
	}

	return result, nil
}

func formatReport(from time.Time, report earnings.Report) string {
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

	return result.String()
}

func (b *Bot) handleChangePay(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
	_ func(string),
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

func (b *Bot) handleResync(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
	_ func(string),
) (string, error) {
	if err := b.resync.Resync(ctx); err != nil {
		return "", err
	}

	return "Resync done.", nil
}

func (b *Bot) handleCommands(
	ctx context.Context,
	i *discordgo.InteractionCreate,
	data discordgo.ApplicationCommandInteractionData,
	initialText func(string),
) (string, error) {
	from, to, err := monthRange(optionalString(data, "month"))
	if err != nil {
		return "", err
	}

	if cached, err := b.earnings.ReadReport(ctx, from, to); err == nil {
		initialText(formatCommands(cached) + "\n-# Refreshing with latest edits...")
	}

	report, err := b.earnings.Report(ctx, from, to)
	if err != nil {
		return "", err
	}

	return formatCommands(report), nil
}

func formatCommands(report earnings.Report) string {
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
	return result.String()
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
			return time.Time{}, time.Time{}, ErrWrongMonthLayout
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

// optionalLanguage reads a language choice, falling back when the option was
// left out. Discord only sends values the command was registered with, so an
// unrecognised one means the command definition and this have drifted apart.
func optionalLanguage(
	data discordgo.ApplicationCommandInteractionData,
	name string,
	fallback entity.Language,
) (entity.Language, error) {
	code := optionalString(data, name)
	if code == "" {
		return fallback, nil
	}

	lang, ok := entity.ParseLanguage(code)
	if !ok {
		return 0, fmt.Errorf("discord: %s: unknown language %q", name, code)
	}

	return lang, nil
}

func optionalBool(data discordgo.ApplicationCommandInteractionData, name string) bool {
	opt := data.GetOption(name)
	if opt == nil {
		return false
	}

	return opt.BoolValue()
}

func monthToText(timestamp time.Time) string {
	return timestamp.Format("January 2006")
}

func earningsToString(amount int) string {
	if amount < 0 {
		panic(fmt.Sprintf("earningsToString: amount = %d", amount))
	}

	daysOfPrem := pricing.DaysPremium(amount)

	switch daysOfPrem {
	case 0:
		return fmt.Sprintf("%d 💎", amount)
	case 1:
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
