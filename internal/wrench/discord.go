package wrench

import (
	"bytes"
	"fmt"
	"path"
	"strings"

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
		b.logf("discord: disconnected")
	})
	firstConnection := true
	b.discordSession.AddHandler(func(s *discordgo.Session, d *discordgo.Connect) {
		if firstConnection {
			firstConnection = false
			return
		}
		b.logf("discord: reconnected")
	})

	// In this example, we only care about receiving message events.
	b.discordSession.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentMessageContent |
		discordgo.IntentDirectMessages

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

func (b *Bot) discordOnMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) error {
	// Ignore all messages created by the bot itself.
	if m.Author.ID == s.State.User.ID {
		return nil
	}

	// Relay all activity to the #activity channel
	if b.Config.ActivityChannel != "disabled" {
		var embeds []*discordgo.MessageEmbed
		embeds = append(embeds, &discordgo.MessageEmbed{
			Color:       3134534,
			Description: "$DESCRIPTION",
			Title:       "Relay",
			Author: &discordgo.MessageEmbedAuthor{
				Name:    fmt.Sprintf("@%s in %v", m.Author.Username, m.Reference().MessageID),
				URL:     "https://discord.com/channels/" + path.Join(fmt.Sprint(m.GuildID), fmt.Sprint(m.ChannelID), fmt.Sprint(m.MessageReference.MessageID)),
				IconURL: m.Author.AvatarURL("32"),
			},
		})
		if len(m.Embeds) > 0 {
			embeds = append(embeds, m.Embeds...)
		}
		err := b.discordSendMessageToChannelEmbeds(b.Config.ActivityChannel, m.Embeds)
		if err != nil {
			b.idLogf("discord-relay", "unable to relay message: %v", err)
		}
	}

	// Handle !wrench comments
	fields := strings.Fields(m.Content)
	if len(fields) >= 2 && fields[0] == "!wrench" {
		cmd := fields[1]
		args := fields[1:]
		if handler, ok := b.discordCommands[cmd]; ok {
			response := handler(args[1:]...)
			if response != "" {
				_, err := s.ChannelMessageSend(m.ChannelID, response)
				if err != nil {
					_, _ = s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
						Title:       "error",
						Description: err.Error(),
					})
					return err
				}
			}
			return nil
		}
		if handler, ok := b.discordCommandsEmbed[cmd]; ok {
			response := handler(args[1:]...)
			if response != nil {
				if response.Description == "" {
					response.Description = "(empty)"
				}
				if len(response.Description) > 4096 {
					response.Description = response.Description[:4096] // Discord limit
				}
				_, err := s.ChannelMessageSendEmbed(m.ChannelID, response)
				if err != nil {
					_, _ = s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
						Title:       "error",
						Description: err.Error(),
					})
					return err
				}
			}
			return nil
		}
		if handler, ok := b.discordCommandsEmbedSecure[cmd]; ok {
			blocked := true
			for _, allowed := range []string{"slimsag"} {
				if m.Author.Username == allowed {
					blocked = false
					break
				}
			}
			if blocked {
				_, err := s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
					Title:       "Forbidden",
					Description: fmt.Sprintf("You are not allowed to run this command '%s'.", m.Author.Username),
				})
				return err
			}
			response := handler(args[1:]...)
			if response != nil {
				if response.Description == "" {
					response.Description = "(empty)"
				}
				if len(response.Description) > 4096 {
					response.Description = response.Description[:4096] // Discord limit
				}
				_, err := s.ChannelMessageSendEmbed(m.ChannelID, response)
				if err != nil {
					_, _ = s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
						Title:       "error",
						Description: err.Error(),
					})
					return err
				}
			}
			return nil
		}
		_, err := s.ChannelMessageSendEmbed(m.ChannelID, b.discordHelp())
		return err
	} else if len(fields) >= 1 && fields[0] == "!wrench" {
		_, err := s.ChannelMessageSendEmbed(m.ChannelID, b.discordHelp())
		return err
	}
	return nil
}

func (b *Bot) discordHelp() *discordgo.MessageEmbed {
	var buf bytes.Buffer
	for _, pair := range b.discordCommandHelp {
		cmd, help := pair[0], pair[1]
		fmt.Fprintf(&buf, "* !wrench %s - %s\n", cmd, help)
	}
	return &discordgo.MessageEmbed{
		Title:       "Available commands",
		Description: buf.String(),
	}
}

func (b *Bot) discordSendMessageToChannel(dstChannel string, message string) error {
	// Get channels for the guild
	channels, err := b.discordSession.GuildChannels(b.Config.DiscordGuildID)
	if err != nil {
		return errors.Wrap(err, "GuildChannels")
	}
	for _, c := range channels {
		// Check if channel is a guild text channel and not a voice or DM channel
		if c.Type != discordgo.ChannelTypeGuildText {
			continue
		}
		if c.Name != dstChannel {
			continue
		}
		_, err := b.discordSession.ChannelMessageSend(c.ID, message)
		if err != nil {
			return errors.Wrap(err, "ChannelMessageSend")
		}
		return nil
	}
	b.logf("discord: unable to find destination channel: %v", dstChannel)
	return nil
}

func (b *Bot) discordSendMessageToChannelEmbeds(dstChannel string, embeds []*discordgo.MessageEmbed) error {
	// Get channels for the guild
	channels, err := b.discordSession.GuildChannels(b.Config.DiscordGuildID)
	if err != nil {
		return errors.Wrap(err, "GuildChannels")
	}
	for _, c := range channels {
		// Check if channel is a guild text channel and not a voice or DM channel
		if c.Type != discordgo.ChannelTypeGuildText {
			continue
		}
		if c.Name != dstChannel {
			continue
		}
		_, err := b.discordSession.ChannelMessageSendEmbeds(c.ID, embeds)
		if err != nil {
			return errors.Wrap(err, "ChannelMessageSendEmbeds")
		}
		return nil
	}
	b.logf("discord: unable to find destination channel: %v", dstChannel)
	return nil
}

func (b *Bot) discord(format string, v ...any) {
	b.logf(format, v...)
	msg := fmt.Sprintf(format, v...)
	err := b.discordSendMessageToChannel(b.Config.DiscordChannel, msg)
	if err != nil {
		b.logf("discord: failed to send message: %v: '%s'", err, msg)
	}
}
