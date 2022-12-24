package wrench

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/hexops/wrench/internal/wrench/api"
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

	b.discordCommandHelp = append(b.discordCommandHelp, [2]string{"ping [id]", "ping a runner to test it is online"})
	b.discordCommandsEmbed["ping"] = func(args ...string) *discordgo.MessageEmbed {
		if len(args) == 0 {
			return &discordgo.MessageEmbed{
				Title:       "ping - error",
				Description: "expected runner ID (see !wrench runners)",
			}
		}

		ctx := context.Background()
		if msg := b.validateRunnerID(ctx, args[0]); msg != nil {
			return msg
		}

		jobTitle := "ping test"
		job, err := b.store.NewRunnerJob(ctx, api.Job{
			Title:          jobTitle,
			TargetRunnerID: args[0],
			Payload: api.JobPayload{
				Ping: true,
			},
		})
		b.idLogf(job.LogID(), "job created: %v", jobTitle)
		if err != nil {
			return &discordgo.MessageEmbed{
				Title:       "ping - error",
				Description: err.Error(),
			}
		}

		start := time.Now()
		for {
			logs, err := b.store.Logs(ctx, job.LogID())
			if err != nil {
				return &discordgo.MessageEmbed{
					Title:       "ping - error",
					Description: "could not read logs after job creation: " + err.Error(),
				}
			}
			for _, log := range logs {
				if strings.Contains(log.Message, "PING SUCCESS") {
					return &discordgo.MessageEmbed{
						Title:       "Ping",
						URL:         b.Config.ExternalURL + "/runners",
						Description: fmt.Sprintf("Ping success! %v/logs/job-%v", b.Config.ExternalURL, job),
					}
				}
			}
			if time.Since(start) > 10*time.Second {
				return &discordgo.MessageEmbed{
					Title:       "ping - timeout waiting for ping success after 10s",
					URL:         b.Config.ExternalURL + "/runners",
					Description: fmt.Sprintf("job created: %v/logs/job-%v", b.Config.ExternalURL, job),
				}
			}
			time.Sleep(1 * time.Second)
		}
	}

	b.discordCommandHelp = append(b.discordCommandHelp, [2]string{"test [runner id] gist", "test execution of a gist on a runner"})
	b.discordCommandsEmbedSecure["test"] = func(args ...string) *discordgo.MessageEmbed {
		if len(args) != 2 {
			return &discordgo.MessageEmbed{
				Title:       "test - error",
				Description: "expected [runner id] [gist] parameters (see !wrench runners for runner ID)",
			}
		}
		runnerID := args[0]
		gist := args[1]

		ctx := context.Background()
		if msg := b.validateRunnerID(ctx, runnerID); msg != nil {
			return msg
		}

		jobTitle := fmt.Sprintf("test %s", gist)
		job, err := b.store.NewRunnerJob(ctx, api.Job{
			Title:          jobTitle,
			TargetRunnerID: runnerID,
			Payload: api.JobPayload{
				Cmd: []string{"script", "test", gist},
			},
		})
		b.idLogf(job.LogID(), "job created: %v", jobTitle)
		b.idLogf(job.LogID(), "testing gist: %v", gist)
		if err != nil {
			return &discordgo.MessageEmbed{
				Title:       "test - error",
				Description: err.Error(),
			}
		}
		return &discordgo.MessageEmbed{
			Title:       "Test gist",
			URL:         b.Config.ExternalURL + "/runners",
			Description: fmt.Sprintf("Job created: %v/logs/job-%v", b.Config.ExternalURL, job),
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

func (b *Bot) validateRunnerID(ctx context.Context, runnerID string) *discordgo.MessageEmbed {
	runners, err := b.store.Runners(ctx)
	if err != nil {
		return &discordgo.MessageEmbed{
			Title:       "error",
			Description: err.Error(),
		}
	}
	found := false
	for _, runner := range runners {
		if runner.ID == runnerID {
			found = true
		}
	}
	if !found {
		return &discordgo.MessageEmbed{
			Title:       "error",
			Description: "invalid runner ID (see !wrench runners)",
		}
	}
	return nil
}
