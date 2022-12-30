package wrench

import (
	"context"
	"time"

	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
)

const schedulerLogID = "scheduler"

type ScheduledJob struct {
	Always bool
	Every  time.Duration
	Job    api.Job
}

func (b *Bot) schedulerStart() error {
	b.schedule = []ScheduledJob{
		{
			Always: true,
			Job: api.Job{
				Title:          "github-runner",
				TargetRunnerID: "darwin-arm64",
				Payload: api.JobPayload{
					Background: true,
					Cmd:        []string{"script", "github-runner"},
					SecretIDs: []string{
						"darwin-arm64/github-runner-url",
						"darwin-arm64/github-runner-token",
					},
				},
			},
		},
		// {
		// 	Every: 5 * time.Minute,
		// 	Job: api.Job{
		// 		Title:          "update-runners",
		// 		TargetRunnerID: "*",
		// 		Payload: api.JobPayload{
		// 			Cmd: []string{"script", "rebuild"},
		// 		},
		// 	},
		// },
		{
			Every: 12 * time.Hour,
			Job: api.Job{
				Title:          "update to latest Zig version",
				TargetRunnerID: "linux-amd64",
				Payload: api.JobPayload{
					Cmd:               []string{"script", "mach-push-rewrite-zig-version"},
					GitPushBranchName: "wrench/update-zig",
					Background:        true, // lightweight enough
					PRTemplate: api.PRTemplate{
						Title: "all: update to latest Zig version",
						Head:  "wrench/update-zig",
						Base:  "main",
						Body: `This change updates us to the latest Zig version.

I'll keep updating this PR so it remains up-to-date until you want to merge it.

Here's the work I did to produce this: ${JOB_LOGS_URL}

- _Wrench the Machanist_
						`,
					},
				},
			},
		},
	}
	go func() {
		ctx := context.Background()
		for {
			time.Sleep(15 * time.Second)
			if err := b.schedulerWork(ctx); err != nil {
				b.idLogf(schedulerLogID, "failed to schedule work: %v", err)
			}
		}
	}()
	return nil
}

func (b *Bot) schedulerWork(ctx context.Context) error {
	runners, err := b.store.Runners(ctx)
	if err != nil {
		return errors.Wrap(err, "Runners")
	}

	scheduleJob := func(schedule ScheduledJob) {
		var filters []JobsFilter
		if schedule.Job.TargetRunnerID != "" {
			filters = append(filters, JobsFilter{TargetRunnerID: schedule.Job.TargetRunnerID})
		}
		lastJob, err := b.lastJobWithTitle(ctx, schedule.Job.Title, filters...)
		if err != nil {
			b.idLogf(schedulerLogID, "failed to query last job: %v", err)
			return
		}

		start := lastJob == nil || (lastJob.State != api.JobStateReady &&
			lastJob.State != api.JobStateStarting &&
			lastJob.State != api.JobStateRunning)
		if !start {
			return
		}

		// Job is not running/scheduled, and is set to Always run OR is a ScheduledStart.
		if !schedule.Always {
			if lastJob == nil || lastJob.State == api.JobStateError {
				schedule.Job.ScheduledStart = time.Now().Add(30 * time.Second)
			} else {
				schedule.Job.ScheduledStart = time.Now().Add(schedule.Every)
			}
		}

		_, err = b.store.NewRunnerJob(ctx, schedule.Job)
		if err != nil {
			b.idLogf(schedulerLogID, "failed to create job: %v: %v", schedule.Job.Title, err)
			return
		}
		b.idLogf(schedulerLogID, "job created: %v", schedule.Job.Title)
	}

	for _, schedule := range b.schedule {
		if schedule.Job.TargetRunnerID == "*" {
			for _, runner := range runners {
				schedule.Job.TargetRunnerID = runner.ID
				scheduleJob(schedule)
				continue
			}
		} else {
			scheduleJob(schedule)
			continue
		}
	}

	return nil
}

func (b *Bot) lastJobWithTitle(ctx context.Context, title string, filters ...JobsFilter) (*api.Job, error) {
	lastJobs, err := b.store.Jobs(ctx, append([]JobsFilter{{Title: title}}, filters...)...)
	if err != nil {
		return nil, errors.Wrap(err, "Jobs")
	}
	if len(lastJobs) == 0 {
		return nil, nil
	}
	lastJob := lastJobs[0]
	for _, job := range lastJobs {
		if job.Created.After(lastJob.Created) {
			lastJob = job
		}
	}
	return &lastJob, nil
}

func (b *Bot) schedulerStop() error {
	return nil
}
