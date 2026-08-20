package discord

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "hello",
		Description: "Says hello!",
	},
}

type Bot struct {
	session *discordgo.Session
}

// New creates a session and wires up handlers. It doesn't connect yet — call Run for that.
func New(token string) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord: new session: %w", err)
	}

	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent

	bot := &Bot{session: session}
	session.AddHandler(bot.handleReady)
	session.AddHandler(bot.handleInteractionCreate)
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

func (b *Bot) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	if i.ApplicationCommandData().Name != "hello" {
		return
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Hello World!",
		},
	})
	if err != nil {
		log.Printf("discord: respond to /hello: %v", err)
	}
}

// handleMessageCreate logs every non-bot message to the console. It ignores
// the bot's own messages so replies don't loop back into the log as noise.
func (b *Bot) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	log.Printf("discord: [%s] %s: %s", m.ChannelID, m.Author.Username, m.Content)
}
