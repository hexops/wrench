package wrench

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) registerCommands() {
	b.discordCommandsEmbed["logs"] = func(args ...string) *discordgo.MessageEmbed {
		logIDs, err := b.store.LogIDs(context.TODO())
		if err != nil {
			return &discordgo.MessageEmbed{
				Title:       "Logs - error",
				Description: err.Error(),
			}
		}

		var buf bytes.Buffer
		for _, id := range logIDs {
			fmt.Fprintf(&buf, "* %s: %s/logs/%s\n", id, b.Config.ExternalURL, id)
		}
		return &discordgo.MessageEmbed{
			Title:       "Logs",
			URL:         b.Config.ExternalURL + "/logs",
			Description: buf.String(),
		}
	}

	b.discordCommandsEmbed["runners"] = func(args ...string) *discordgo.MessageEmbed {
		runners, err := b.store.Runners(context.TODO())
		if err != nil {
			return &discordgo.MessageEmbed{
				Title:       "Runners - error",
				Description: err.Error(),
			}
		}

		var buf bytes.Buffer
		if len(runners) == 0 {
			fmt.Fprintf(&buf, "no runners found\n")
		}
		for _, runner := range runners {
			fmt.Fprintf(&buf, "* **'%v' (%v)** (last seen %v ago)\n", runner.ID, runner.Arch, time.Since(runner.LastSeenAt).Round(time.Second))
		}
		return &discordgo.MessageEmbed{
			Title: "Runners",
			// TODO: link to better runner overview pagw when there is one
			URL:         b.Config.ExternalURL,
			Description: buf.String(),
		}
	}

	b.discordCommandsEmbed["version"] = func(args ...string) *discordgo.MessageEmbed {
		return &discordgo.MessageEmbed{
			Title:       "wrench @ " + Version,
			URL:         "https://github.com/hexops/wrench/commit/" + Version,
			Description: fmt.Sprintf("* `%s` (%s)\n* %s\n* %s", Version, CommitTitle, Date, GoVersion),
		}
	}
}
