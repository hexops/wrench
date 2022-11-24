package wrench

import (
	"bytes"
	"context"
	"fmt"

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

	b.discordCommandsEmbed["version"] = func(args ...string) *discordgo.MessageEmbed {
		return &discordgo.MessageEmbed{
			Title:       "wrench @ " + Version,
			URL:         "https://github.com/hexops/wrench/commit/" + Version,
			Description: fmt.Sprintf("* `%s` (%s)\n* %s\n* %s", Version, CommitTitle, Date, GoVersion),
		}
	}
}
