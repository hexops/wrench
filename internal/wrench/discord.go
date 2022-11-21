package wrench

import (
	"github.com/bwmarrin/discordgo"
	"github.com/hexops/wrench/internal/errors"
)

func (b *Bot) discordStart() error {
	if b.Config.DiscordBotToken == "" {
		b.logf("discord: disabled (config.DiscordBotToken not configured)")
		return nil
	}
	if b.Config.DiscordGuildID == "" {
		return errors.New("discord: config.DiscordGuildID not configured but is required if DiscordBotToken present")
	}

	var err error
	b.discordSession, err = discordgo.New("Bot " + b.Config.DiscordBotToken)
	if err != nil {
		return errors.Wrap(err, "New")
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	b.discordSession.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if err := b.discordOnMessageCreate(s, m); err != nil {
			b.logf("discord: message create: %v", err)
		}
	})
	b.discordSession.AddHandler(func(s *discordgo.Session, d *discordgo.Disconnect) {
		if err := b.discordOnDisconnect(s, d); err != nil {
			b.logf("discord: disconnect: %v", err)
		}
	})

	// In this example, we only care about receiving message events.
	b.discordSession.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentMessageContent

	// Open a websocket connection to Discord and begin listening.
	err = b.discordSession.Open()
	if err != nil {
		return errors.Wrap(err, "Open")
	}
	return nil
}

func (b *Bot) discordStop() error {
	if b.discordSession == nil {
		return nil
	}
	return b.discordSession.Close()
}

func (b *Bot) discordOnDisconnect(s *discordgo.Session, d *discordgo.Disconnect) error {
	b.logf("discord: disconnected, trying to reconnect")
	err := b.discordSession.Open()
	if err != nil {
		return errors.Wrap(err, "Open")
	}
	b.logf("discord: reconnected!")
	return nil
}

func (b *Bot) discordOnMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) error {
	// Ignore all messages created by the bot itself.
	if m.Author.ID == s.State.User.ID {
		return nil
	}

	if m.Content == "wrench?" {
		s.ChannelMessageSend(m.ChannelID, "yes?")
	}
	return nil
}
