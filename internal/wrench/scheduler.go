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
		{
			Every: 5 * time.Minute,
			Job: api.Job{
				Title:          "update-runners",
				TargetRunnerID: "*",
				Payload: api.JobPayload{
					Cmd: []string{"script", "rebuild"},
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
	activeJobs, err := b.store.Jobs(ctx,
		// Starting OR Running
		JobsFilter{NotState: api.JobStateSuccess},
		JobsFilter{NotState: api.JobStateError},
		JobsFilter{NotState: api.JobStateReady},
	)
	if err != nil {
		return errors.Wrap(err, "Jobs")
	}

scheduling:
	for _, schedule := range b.schedule {
		start := false
		if schedule.Always {
			for _, job := range activeJobs {
				if job.Title == schedule.Job.Title {
					// already exists
					continue scheduling
				}
			}
			start = true
		} else if schedule.Every != 0 {
			lastJob, err := b.lastJobWithTitle(ctx, schedule.Job.Title)
			if err != nil {
				b.idLogf(schedulerLogID, "failed to query last job: %v", err)
				continue
			}
			start = lastJob == nil || (lastJob.State != api.JobStateReady &&
				lastJob.State != api.JobStateStarting &&
				lastJob.State != api.JobStateRunning &&
				time.Since(lastJob.Created) > schedule.Every)
			schedule.Job.ScheduledStart = time.Now().Add(schedule.Every)
		}

		if start {
			if schedule.Job.TargetRunnerID == "*" {
				runners, err := b.store.Runners(ctx)
				if err != nil {
					b.idLogf(schedulerLogID, "failed to query runners: %v", err)
					continue
				}
				for _, runner := range runners {
					schedule.Job.TargetRunnerID = runner.ID
					_, err := b.store.NewRunnerJob(ctx, schedule.Job)
					if err != nil {
						b.idLogf(schedulerLogID, "failed to create job: %v: %v", schedule.Job.Title, err)
						continue
					}
					b.idLogf(schedulerLogID, "job created: %v", schedule.Job.Title)
				}
			} else {
				_, err := b.store.NewRunnerJob(ctx, schedule.Job)
				if err != nil {
					b.idLogf(schedulerLogID, "failed to create job: %v: %v", schedule.Job.Title, err)
					continue
				}
				b.idLogf(schedulerLogID, "job created: %v", schedule.Job.Title)
			}
		}
	}

	return nil
}

func (b *Bot) lastJobWithTitle(ctx context.Context, title string) (*api.Job, error) {
	lastJobs, err := b.store.Jobs(ctx, JobsFilter{Title: title})
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
