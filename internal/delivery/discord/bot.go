package discord

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/usecase/earnings"
	"github.com/the-new-day/wiki-earnings/internal/usecase/resync"
	"github.com/the-new-day/wiki-earnings/internal/usecase/revisions"
	"github.com/the-new-day/wiki-earnings/internal/usecase/tasks"
)

var commands = []*discordgo.ApplicationCommand{}

func init() {
	registerCommands()
}

// TaskConfig is what the bot needs to serve /task: how to translate the text,
// which languages to translate it into, and where each language's text goes.
type TaskConfig struct {
	Translator tasks.Translator
	Languages  []entity.Language
	Channels   map[entity.Language][]string
}

type Bot struct {
	session *discordgo.Session

	earnings  *earnings.UseCase
	revisions *revisions.UseCase
	resync    *resync.UseCase
	tasks     *tasks.UseCase

	wikiRoleID      string
	wikiAdminRoleID string

	taskChannels map[entity.Language][]string
}

// New creates a session and wires up handlers. It doesn't connect yet — call Run for that.
func New(
	token string,
	earningsUC *earnings.UseCase,
	revisionsUC *revisions.UseCase,
	resyncUC *resync.UseCase,
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
		revisions:       revisionsUC,
		resync:          resyncUC,
		wikiRoleID:      wikiRoleID,
		wikiAdminRoleID: wikiAdminRoleID,
		taskChannels:    taskCfg.Channels,
	}

	// The bot is both what triggers a task and where it lands, so it builds the
	// use case itself. Wiring it from outside would need the bot to exist first
	// and then be handed the use case, leaving a window where /task panics.
	bot.tasks = tasks.New(taskCfg.Translator, bot, taskCfg.Languages)
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
