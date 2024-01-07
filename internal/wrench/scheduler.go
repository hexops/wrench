package wrench

import (
	"context"
	"strings"
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
				ID:             "github-runner",
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
			Every: 24 * time.Hour,
			Job: api.Job{
				ID:             "web-check-assets",
				Title:          "website: check asset URLs",
				TargetRunnerID: "linux-amd64",
				Payload: api.JobPayload{
					Cmd: []string{"script", "web-check-assets"},
				},
			},
		},
		{
			Every: 24 * time.Hour,
			Job: api.Job{
				ID:             "web-check-broken-urls",
				Title:          "website: check for broken URLs",
				TargetRunnerID: "linux-amd64",
				Payload: api.JobPayload{
					Cmd: []string{"script", "web-check-broken-urls"},
				},
			},
		},
		{
			Every: 24 * time.Hour,
			Job: api.Job{
				ID:             "stat-mach-core",
				Title:          "mach-core: calculate build stats",
				TargetRunnerID: "linux-amd64",
				Payload: api.JobPayload{
					Cmd: []string{"script", "stat-mach-core"},
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
			Every: 0,
			Job: api.Job{
				ID:             "update-zig-version",
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

\- _Wrench the Machanist_
								`,
					},
				},
			},
		},
		// 		{
		// 			Every: 24 * time.Hour,
		// 			Job: api.Job{
		// 				ID:             "update-deps",
		// 				Title:          "update build.zig.zon dependencies",
		// 				TargetRunnerID: "linux-amd64",
		// 				Payload: api.JobPayload{
		// 					Cmd:               []string{"script", "push-update-deps"},
		// 					GitPushBranchName: "wrench/update-deps",
		// 					Background:        true, // lightweight enough
		// 					PRTemplate: api.PRTemplate{
		// 						Title: "all: update build.zig.zon dependencies",
		// 						Head:  "wrench/update-deps",
		// 						Base:  "main",
		// 						Body: `This change updates build.zig.zon to the latest version of dependencies.

		// I'll keep updating this PR so it remains up-to-date until you want to merge it.

		// Here's the work I did to produce this: ${JOB_LOGS_URL}

		// \- _Wrench the Machanist_
		// 						`,
		// 					},
		// 				},
		// 			},
		// 		},
		{
			Every: 7 * 24 * time.Hour,
			Job: api.Job{
				ID:             "gpu-dawn-update-dawn-version",
				Title:          "gpu-dawn: update to latest Dawn version",
				TargetRunnerID: "darwin-amd64",
				Payload: api.JobPayload{
					Cmd:               []string{"script", "mach-update-gpu-dawn"},
					GitPushBranchName: "wrench/update-gpu-dawn",
					PRTemplate: api.PRTemplate{
						Title: "gpu-dawn: update to latest Dawn version",
						Head:  "wrench/update-gpu-dawn",
						Base:  "main",
						Body: strings.ReplaceAll(`This change updates libs/gpu-dawn to use latest Dawn version '${METADATA_NEWBRANCH}'

The WebGPU API may have changed, review these diffs to see if 'libs/gpu' needs to be updated:

* [ ] ['webgpu.h' header diff](${CUSTOM_LOG_DAWN_DIFF_HEADER})
* [ ] [dawn.json diff](${CUSTOM_LOG_DAWN_DIFF_JSON})

Note:

* Once merged, the [mach-gpu-dawn](https://github.com/hexops/mach-gpu-dawn) CI pipeline will produce binary releases and update 'libs/gpu' in this repository to begin using this new version.
* If the mach-gpu-dawn CI fails, you may want to review the [Dawn build file changes](${CUSTOM_LOG_DAWN_DIFF_BUILD}) to see if 'gpu-dawn/build.zig' needs updates.
* I'll keep updating this PR so it remains up-to-date until you want to merge it.

The work I did to produce this can be viewed here: ${JOB_LOGS_URL}

\- _Wrench the Machanist_
`, "'", "`"),
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

	for _, schedule := range b.schedule {
		if _, err := b.scheduleJob(ctx, schedule, runners, false); err != nil {
			b.idLogf(schedulerLogID, "%v", err)
			continue
		}
	}
	return nil
}

func (b *Bot) scheduleJobNow(ctx context.Context, scheduledJobID api.JobID, runners []api.Runner) (api.JobID, error) {
	var found *ScheduledJob
	for _, scheduled := range b.schedule {
		if scheduled.Job.ID == scheduledJobID {
			found = &scheduled
			break
		}
	}
	if found == nil {
		return "", errors.New("scheduled job not found")
	}
	return b.scheduleJob(ctx, *found, runners, true)
}

func (b *Bot) scheduleJob(ctx context.Context, schedule ScheduledJob, runners []api.Runner, force bool) (api.JobID, error) {
	if schedule.Job.TargetRunnerID == "*" {
		for _, runner := range runners {
			schedule.Job.TargetRunnerID = runner.ID
			break
		}
	}

	var filters []JobsFilter
	if schedule.Job.TargetRunnerID != "" {
		filters = append(filters, JobsFilter{TargetRunnerID: schedule.Job.TargetRunnerID})
	}
	lastJob, err := b.lastJobWithTitle(ctx, schedule.Job.Title, filters...)
	if err != nil {
		return "", errors.Wrap(err, "failed to query last job")
	}

	startable := lastJob == nil || (lastJob.State != api.JobStateReady &&
		lastJob.State != api.JobStateStarting &&
		lastJob.State != api.JobStateRunning &&
		lastJob.State != api.JobStateError)
	lastJobErrored := lastJob != nil && lastJob.State == api.JobStateError
	shouldStart := startable || lastJobErrored
	if startable && schedule.Every == 0 {
		shouldStart = false // Job can be started, but is not scheduled to start automatically.
	}

	if force {
		if startable {
			lastJob.ScheduledStart = time.Time{}
			if err := b.store.UpsertRunnerJob(ctx, *lastJob); err != nil {
				return "", errors.Wrap(err, "failed to update job")
			}
			return lastJob.ID, nil
		} else {
			schedule.Job.ScheduledStart = time.Time{}
			jobID, err := b.store.NewRunnerJob(ctx, schedule.Job)
			if err != nil {
				return "", errors.Wrap(err, "failed to create job")
			}
			b.idLogf(schedulerLogID, "job created: %v", schedule.Job.Title)
			return jobID, nil
		}
	}
	if !shouldStart {
		return "", nil
	}

	// Job is not running/scheduled, and is set to Always run OR is a ScheduledStart.
	if !schedule.Always {
		if lastJob == nil || lastJobErrored {
			schedule.Job.ScheduledStart = time.Now().Add(30 * time.Second)
		} else {
			schedule.Job.ScheduledStart = time.Now().Add(schedule.Every)
		}
	}

	jobID, err := b.store.NewRunnerJob(ctx, schedule.Job)
	if err != nil {
		return "", errors.Wrap(err, "failed to create job")
	}
	b.idLogf(schedulerLogID, "job created: %v", schedule.Job.Title)
	return jobID, nil
}

func (b *Bot) cancelJob(ctx context.Context, scheduledJobID api.JobID, runners []api.Runner) (api.JobID, error) {
	var schedule *ScheduledJob
	for _, scheduled := range b.schedule {
		if scheduled.Job.ID == scheduledJobID {
			schedule = &scheduled
			break
		}
	}
	if schedule == nil {
		return "", errors.New("scheduled job not found")
	}

	var filters []JobsFilter
	if schedule.Job.TargetRunnerID != "" {
		filters = append(filters, JobsFilter{TargetRunnerID: schedule.Job.TargetRunnerID})
	}
	lastJob, err := b.lastJobWithTitle(ctx, schedule.Job.Title, filters...)
	if err != nil {
		return "", errors.Wrap(err, "failed to query last job")
	}

	b.idLogf("job-"+string(lastJob.ID), "error: job cancelled")
	b.idLogf(schedulerLogID, "job cancelled: %v", schedule.Job.Title)
	lastJob.State = api.JobStateError
	lastJob.ScheduledStart = time.Time{}
	if err := b.store.UpsertRunnerJob(ctx, *lastJob); err != nil {
		return "", errors.Wrap(err, "failed to update job")
	}
	return lastJob.ID, nil
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
