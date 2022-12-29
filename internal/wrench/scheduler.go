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
		}

		if start {
			_, err := b.store.NewRunnerJob(ctx, schedule.Job)
			if err != nil {
				b.idLogf(schedulerLogID, "failed to create job: %v: %v", schedule.Job.Title, err)
				continue
			}
			b.idLogf(schedulerLogID, "job created: %v", schedule.Job.Title)
		}
	}

	return nil
}

func (b *Bot) schedulerStop() error {
	return nil
}
