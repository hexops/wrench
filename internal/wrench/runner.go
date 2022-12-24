package wrench

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
)

func (b *Bot) runnerStart() error {
	if b.Config.Runner == "" {
		return errors.New("runner: Config.Runner must be configured")
	}
	if b.Config.ExternalURL == "" {
		return errors.New("runner: Config.ExternalURL must be configured")
	}
	if b.Config.Secret == "" {
		return errors.New("runner: Config.Secret must be configured")
	}
	arch := runtime.GOOS + "/" + runtime.GOARCH
	b.runner = &api.Client{URL: b.Config.ExternalURL, Secret: b.Config.Secret}

	connected := false
	go func() {
		var (
			activeMu  sync.RWMutex
			active    *api.Job
			activeLog bytes.Buffer
		)

		env := api.RunnerEnv{
			WrenchVersion:     Version,
			WrenchCommitTitle: CommitTitle,
			WrenchDate:        Date,
			WrenchGoVersion:   GoVersion,
		}

		logID := "runner"
		started := false
		for {
			if started {
				time.Sleep(5 * time.Second)
			}
			started = true
			ctx := context.Background()
			activeMu.Lock()
			var update *api.RunnerJobUpdate
			if active != nil {
				update = &api.RunnerJobUpdate{
					ID:     active.ID,
					State:  active.State,
					Log:    activeLog.String(),
					Pushed: false, // TODO: pushing
				}
			}
			resp, err := b.runner.RunnerPoll(ctx, &api.RunnerPollRequest{
				ID:   b.Config.Runner,
				Arch: arch,
				Job:  update,
				Env:  env,
			})
			if err == nil {
				if active != nil && (active.State == api.JobStateSuccess || active.State == api.JobStateError) {
					active = nil // job finished
				}
				activeLog.Reset()
			}
			activeMu.Unlock()
			if !connected {
				connected = true
				b.idLogf(logID, "working for %s ('%s', %s)", b.Config.ExternalURL, b.Config.Runner, arch)
			}
			if err != nil {
				b.idLogf(logID, "error: %v", err)
				continue
			}
			if resp.NotFound {
				b.idLogf(logID, "error: job not found, dropping job")
				active = nil
				continue
			}

			if resp.Start != nil {
				activeMu.Lock()
				active = &api.Job{
					ID:      resp.Start.ID,
					Title:   resp.Start.Title,
					Payload: resp.Start.Payload,
					State:   api.JobStateRunning,
				}
				b.idLogf(logID, "running job: id=%v title=%v", active.ID, active.Title)
				fmt.Fprintf(&activeLog, "running job: id=%v title=%v\n", active.ID, active.Title)
				activeMu.Unlock()

				go func() {
					if active.Payload.Ping {
						activeMu.Lock()
						active.State = api.JobStateSuccess
						fmt.Fprintf(&activeLog, "PING SUCCESS (job id=%v)\n", active.ID)
						activeMu.Unlock()
						return
					}
					err := b.runWrench(&activeLog, active.Payload.Cmd...)
					if err != nil {
						activeMu.Lock()
						active.State = api.JobStateError
						fmt.Fprintf(&activeLog, "ERROR: %v (job id=%v)\n", err, active.ID)
						activeMu.Unlock()
						return
					}
					activeMu.Lock()
					active.State = api.JobStateSuccess
					fmt.Fprintf(&activeLog, "SUCCESS (job id=%v)\n", active.ID)
					activeMu.Unlock()
				}()
			} else {
				b.idLogf(logID, "waiting for a job")
			}
		}
	}()
	return nil
}
