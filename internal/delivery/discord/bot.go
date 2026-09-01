package discord

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/usecase/corrections"
	"github.com/the-new-day/wiki-earnings/internal/usecase/earnings"
	"github.com/the-new-day/wiki-earnings/internal/usecase/editors"
	"github.com/the-new-day/wiki-earnings/internal/usecase/resync"
	"github.com/the-new-day/wiki-earnings/internal/usecase/revisions"
	"github.com/the-new-day/wiki-earnings/internal/usecase/tasks"
)

var commands = []*discordgo.ApplicationCommand{}

func init() {
	registerCommands()
}

// TaskTarget is one locale /task can post to: the language its editors read
// tasks in, and the channel they read them from.
type TaskTarget struct {
	Locale    string
	Language  entity.Language
	ChannelID string
}

// TaskConfig is what the bot needs to serve /task: how to translate the text,
// and which locales it can be sent to. The order of Targets is the order the
// locales are offered in.
type TaskConfig struct {
	Translator tasks.Translator
	Targets    []TaskTarget
}

type Bot struct {
	session *discordgo.Session

	earnings    *earnings.UseCase
	editors     *editors.UseCase
	revisions   *revisions.UseCase
	resync      *resync.UseCase
	corrections *corrections.UseCase
	tasks       *tasks.UseCase

	wikiRoleID      string
	wikiAdminRoleID string

	// taskChannels is where each locale reads tasks. Which locales exist at all
	// is the use case's to say - this only answers where one of them lands.
	taskChannels map[string]string
}

// New creates a session and wires up handlers. It doesn't connect yet — call Run for that.
func New(
	token string,
	earningsUC *earnings.UseCase,
	editorsUC *editors.UseCase,
	revisionsUC *revisions.UseCase,
	resyncUC *resync.UseCase,
	correctionsUC *corrections.UseCase,
	wikiRoleID, wikiAdminRoleID string,
	messageLifetime time.Duration,
	taskCfg TaskConfig,
) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord: new session: %w", err)
	}

	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent

	bot := &Bot{
		session:         session,
		earnings:        earningsUC,
		editors:         editorsUC,
		revisions:       revisionsUC,
		resync:          resyncUC,
		corrections:     correctionsUC,
		wikiRoleID:      wikiRoleID,
		wikiAdminRoleID: wikiAdminRoleID,
		taskChannels:    taskChannels(taskCfg.Targets),
	}

	// The bot is both what triggers a task and where it lands, so it builds the
	// use case itself. Wiring it from outside would need the bot to exist first
	// and then be handed the use case, leaving a window where /task panics.
	bot.tasks = tasks.New(taskCfg.Translator, bot, taskTargets(taskCfg.Targets))
	session.AddHandler(bot.handleReady)
	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		bot.handleInteractionCreate(i, messageLifetime)
	})
	session.AddHandler(bot.handleMessageCreate)

	return bot, nil
}

// Run opens the gateway connection, registers commands, and blocks until ctx
// is cancelled, then tears both down.
func (b *Bot) Run(ctx context.Context) error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("discord: open session: %w", err)
	}
	defer b.session.Close()

	registered, err := b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, "", commands)
	if err != nil {
		return fmt.Errorf("discord: register commands: %w", err)
	}

	<-ctx.Done()
	log.Println("discord: shutting down")

	for _, cmd := range registered {
		if err := b.session.ApplicationCommandDelete(b.session.State.User.ID, "", cmd.ID); err != nil {
			log.Printf("discord: delete command %s: %v", cmd.Name, err)
		}
	}

	return nil
}

func (b *Bot) handleReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("discord: logged in as %s", r.User.String())
}

// handleMessageCreate logs every non-bot message to the console.
// Ignores the bot's own messages.
func (b *Bot) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	log.Printf("discord: [%s] %s: %s", m.ChannelID, m.Author.Username, m.Content)
}

// taskChannels is the delivery half of the targets: which channel a locale's
// task lands in.
func taskChannels(targets []TaskTarget) map[string]string {
	channels := make(map[string]string, len(targets))

	for _, target := range targets {
		channels[target.Locale] = target.ChannelID
	}

	return channels
}

// taskTargets is the half the use case needs: which locales exist and what
// language each one reads.
func taskTargets(targets []TaskTarget) []tasks.Target {
	converted := make([]tasks.Target, 0, len(targets))

	for _, target := range targets {
		converted = append(converted, tasks.Target{Locale: target.Locale, Language: target.Language})
	}

	return converted
}

// languageChoices offers every language the service knowse.
func languageChoices() []*discordgo.ApplicationCommandOptionChoice {
	langs := entity.Languages()
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(langs))

	for _, lang := range langs {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  strings.ToUpper(lang.String()),
			Value: lang.String(),
		})
	}

	return choices
}

func registerCommands() {
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "salary",
			Description: "Editor's salary per month",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "nickname",
					Description: "Editor's nickname on the Wiki",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "month",
					Description: "Month in YYYY-MM format, current by default",
				},
			},
		},
		{
			Name:        "edits",
			Description: "A detailed report on the editor's edits for the month",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "nickname",
					Description: "Editor's nickname on the Wiki",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "show_minor",
					Description: "Show ((ME)) and ((IA)) edits",
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "month",
					Description: "Month in YYYY-MM format, current by default",
				},
			},
		},
		{
			Name:        "task",
			Description: "Post a task for editors",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "text",
					Description: "Text of the task",
					Required:    true,
				},
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "locales",
					Description:  "Locales to post to, comma separated. All of them by default",
					Autocomplete: true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "source_lang",
					Description: "Language the text is written in, Russian by default",
					Choices:     languageChoices(),
				},
			},
		},
		{
			Name:        "report",
			Description: "Full report on all editors for the month",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "month",
					Description: "Month in YYYY-MM format, current by default",
				},
			},
		},
		{
			Name:        "changepay",
			Description: "Manually change the editing cost",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "nickname",
					Description: "Editor's nickname on the Wiki",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "edit_id",
					Description: "Editing ID",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "new_cost",
					Description: "New editing cost",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "locale",
					Description: "Wiki locale (needed if the editor has multiple accounts)",
				},
			},
		},
		{
			Name:        "correction",
			Description: "Add a payment correction for an editor. Give amount or target, not both",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "nickname",
					Description: "Editor's nickname on the Wiki",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "amount",
					Description: "Crystals to add, or subtract if negative",
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "target",
					Description: "Add whatever it takes to bring the month's earnings to this figure",
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "description",
					Description: "Why the correction is made. May be left out",
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "month",
					Description: "Month to book it in, YYYY-MM. Current by default",
				},
			},
		},
		{
			Name:        "removecorrection",
			Description: "Remove a payment correction by its id (shown in /edits)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "id",
					Description: "Correction id, as shown in /edits",
					Required:    true,
				},
			},
		},
		{
			Name:        "paynick",
			Description: "Set the account an editor is paid on",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "nickname",
					Description: "Editor's nickname on the Wiki",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "payments_nickname",
					Description: "In-game nickname to pay. Leave out to pay the Wiki nickname again",
				},
			},
		},
		{
			Name:        "commands",
			Description: "Get chat commands for payments",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "month",
					Description: "Month in YYYY-MM format, current by default",
				},
			},
		},
		{
			Name:        "resync",
			Description: "Reload all edits from the Wiki and recalculate payments. Run ONLY if absolutely necessary",
		},
	}
}
