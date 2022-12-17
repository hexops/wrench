package wrench

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) registerCommands() {
	b.discordCommandHelp = append(b.discordCommandHelp, [2]string{"logs", "show log locations"})
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

	b.discordCommandHelp = append(b.discordCommandHelp, [2]string{"runners", "show known runners"})
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
			Title:       "Runners",
			URL:         b.Config.ExternalURL + "/runners",
			Description: buf.String(),
		}
	}

	b.discordCommandHelp = append(b.discordCommandHelp, [2]string{"prs", "show open pull requests"})
	b.discordCommandsEmbed["prs"] = func(args ...string) *discordgo.MessageEmbed {
		ctx := context.Background()
		var buf bytes.Buffer

		count := 0
		for _, repoPair := range githubRepoNames {
			pullRequests, err := b.githubPullRequests(ctx, repoPair)
			if err != nil {
				return &discordgo.MessageEmbed{
					Title:       "Runners - error",
					Description: err.Error(),
				}
			}

			open := 0
			for _, pr := range pullRequests {
				if *pr.State != "open" {
					continue
				}
				open++
			}
			if open == 0 {
				continue
			}

			fmt.Fprintf(&buf, "**%v**:\n", repoPair)
			for _, pr := range pullRequests {
				if *pr.State != "open" {
					continue
				}
				count++
				fmt.Fprintf(&buf, "* ['%v'](%s) (by _%v_)\n", *pr.Title, *pr.URL, *pr.User.Login)
			}
			fmt.Fprintf(&buf, "\n")
		}
		if count == 0 {
			fmt.Fprintf(&buf, "no pull requests found\n")
		}

		return &discordgo.MessageEmbed{
			Title:       "Open pull requests",
			Description: buf.String(),
		}
	}

	b.discordCommandHelp = append(b.discordCommandHelp, [2]string{"version", "show wrench version"})
	b.discordCommandsEmbed["version"] = func(args ...string) *discordgo.MessageEmbed {
		return &discordgo.MessageEmbed{
			Title:       "wrench @ " + Version,
			URL:         "https://github.com/hexops/wrench/commit/" + Version,
			Description: fmt.Sprintf("* `%s` (%s)\n* %s\n* %s", Version, CommitTitle, Date, GoVersion),
		}
	}
}
