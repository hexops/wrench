package wrench

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
	"github.com/hexops/wrench/internal/wrench/scripts"
	"golang.org/x/exp/slices"
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
	b.runner = &api.Client{URL: b.Config.ExternalURL, Secret: b.Config.Secret}

	go func() {
		arch := runtime.GOOS + "/" + runtime.GOARCH
		connected := false
		env := api.RunnerEnv{
			WrenchVersion:     Version,
			WrenchCommitTitle: CommitTitle,
			WrenchDate:        Date,
			WrenchGoVersion:   GoVersion,
		}
		type runningJob struct {
			ID   api.JobID
			Done chan struct{}
		}
		runningJobs := []runningJob{}

		logID := "runner"
		started := false
		for {
			if started {
				time.Sleep(5 * time.Second)
			}
			started = true
			ctx := context.Background()

		sliceUpdated:
			var runningIDs []api.JobID
			for i, running := range runningJobs {
				select {
				case _, ok := <-running.Done:
					if !ok {
						runningJobs = slices.Delete(runningJobs, i, i+1)
						goto sliceUpdated
					}
				default:
				}
				runningIDs = append(runningIDs, running.ID)
			}

			resp, err := b.runner.RunnerPoll(ctx, &api.RunnerPollRequest{
				ID:      b.Config.Runner,
				Arch:    arch,
				Running: runningIDs,
				Env:     env,
			})
			if !connected {
				connected = true
				b.idLogf(logID, "working for %s ('%s', %s)", b.Config.ExternalURL, b.Config.Runner, arch)
			}
			if err != nil {
				b.idLogf(logID, "error: %v", err)
				continue
			}

			if resp.Start != nil {
				done := make(chan struct{})
				runningJobs = append(runningJobs, runningJob{
					ID:   resp.Start.ID,
					Done: done,
				})
				b.idLogf(logID, "starting job: id=%v title=%v", resp.Start.ID, resp.Start.Title)
				b.runnerStartJob(ctx, resp.Start, done)
			} else {
				b.idLogf(logID, "waiting for jobs, running: %v", runningIDs)
			}
		}
	}()
	return nil
}

type lockedWriter struct {
	mu *sync.RWMutex
	w  io.Writer
}

func (m lockedWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.w.Write(p)
}

func (b *Bot) runnerStartJob(ctx context.Context, startJob *api.RunnerJobStart, done chan struct{}) {
	var (
		activeMu  sync.RWMutex
		active    *api.Job
		activeLog bytes.Buffer
		logID     = "job-" + string(startJob.ID)
		arch      = runtime.GOOS + "/" + runtime.GOARCH
	)

	activeMu.Lock()
	active = &api.Job{
		ID:      startJob.ID,
		Title:   startJob.Title,
		Payload: startJob.Payload,
		State:   api.JobStateRunning,
	}
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

		lw := lockedWriter{mu: &activeMu, w: &activeLog}
		cmd := scripts.NewCmd(lw, "wrench", active.Payload.Cmd, scripts.WorkDir(b.Config.WrenchDir))
		cmd.Stderr = lw
		cmd.Stdout = lw
		err := cmd.Run()
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				err = fmt.Errorf("'wrench': error: exit code: %v", exitError.ExitCode())
			}
		}

		activeMu.Lock()
		defer activeMu.Unlock()
		if err != nil {
			active.State = api.JobStateError
			fmt.Fprintf(&activeLog, "ERROR: %v (job id=%v)\n", err, active.ID)
			return
		}
		active.State = api.JobStateSuccess
		fmt.Fprintf(&activeLog, "SUCCESS (job id=%v)\n", active.ID)
	}()

	go func() {
		for {
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
			resp, err := b.runner.RunnerJobUpdate(ctx, &api.RunnerJobUpdateRequest{
				ID:   b.Config.Runner,
				Arch: arch,
				Job:  update,
			})
			if err == nil {
				if active != nil && (active.State == api.JobStateSuccess || active.State == api.JobStateError) {
					active = nil // job finished
					close(done)
					return
				}
				activeLog.Reset()
			}
			activeMu.Unlock()
			if err != nil {
				b.idLogf(logID, "error: %v", err)
				continue
			}
			if resp.NotFound {
				b.idLogf(logID, "error: job not found, dropping job")
				active = nil
				close(done)
				return
			}
		}
	}()
}
