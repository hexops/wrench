package wrench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
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
			Title        string
			ID           api.JobID
			Cancel, Done chan struct{}
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
				for _, running := range runningJobs {
					if resp.Start.Title == running.Title {
						// We were asked to start a job that is currently running, cancel the old one.
						close(running.Cancel)
						<-running.Done // wait for it to finish
					}
				}

				done := make(chan struct{})
				cancel := make(chan struct{})
				runningJobs = append(runningJobs, runningJob{
					ID:     resp.Start.ID,
					Title:  resp.Start.Title,
					Cancel: cancel,
					Done:   done,
				})
				b.idLogf(logID, "starting job: id=%v title=%v", resp.Start.ID, resp.Start.Title)
				b.runnerStartJob(ctx, resp.Start, cancel, done)
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

func (b *Bot) runnerStartJob(ctx context.Context, startJob *api.RunnerJobStart, cancel, done chan struct{}) {
	var (
		activeMu             sync.RWMutex
		active               *api.Job
		activeLog            bytes.Buffer
		activeScriptResponse *api.ScriptResponse
		logID                = "job-" + string(startJob.ID)
		arch                 = runtime.GOOS + "/" + runtime.GOARCH
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
		opts := []scripts.CmdOption{
			scripts.WorkDir(b.Config.WrenchDir),
			scripts.Env("WRENCH_RUNNER_ID", b.Config.Runner),
		}
		for secretName, secretValue := range startJob.Secrets {
			secretName = strings.Replace(secretName, "/", "_", -1)
			secretName = strings.Replace(secretName, "-", "_", -1)
			secretName = strings.ToUpper(secretName)
			opts = append(opts, scripts.Env("WRENCH_SECRET_"+secretName, secretValue))
		}
		opts = append(opts, scripts.Env("WRENCH_SECRET_GIT_PUSH_USERNAME", startJob.GitPushUsername))
		opts = append(opts, scripts.Env("WRENCH_SECRET_GIT_PUSH_PASSWORD", startJob.GitPushPassword))
		opts = append(opts, scripts.Env("WRENCH_SECRET_GIT_CONFIG_USER_EMAIL", startJob.GitConfigUserEmail))
		opts = append(opts, scripts.Env("WRENCH_SECRET_GIT_CONFIG_USER_NAME", startJob.GitConfigUserName))

		opts = append(opts, scripts.Env("WRENCH_GIT_PUSH_BRANCH_NAME", startJob.Payload.GitPushBranchName))
		var responseBuf bytes.Buffer
		cmd := scripts.NewCmd(lw, "wrench", active.Payload.Cmd, opts...)
		cmd.Stderr = lw
		cmd.Stdout = &responseBuf
		go func() {
			select {
			case <-done:
				return
			case <-cancel:
			}
			cmd.Process.Kill()
		}()
		err := cmd.Run()
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				err = fmt.Errorf("'wrench': error: exit code: %v", exitError.ExitCode())
			}
		}

		activeMu.Lock()
		defer activeMu.Unlock()
		if err == nil {
			var response *api.ScriptResponse
			if err2 := json.NewDecoder(&responseBuf).Decode(&response); err2 != nil {
				err = fmt.Errorf("cannot unmarshal script response JSON (%v): '%s'", err2, responseBuf.String())
			} else {
				activeScriptResponse = response
				if len(activeScriptResponse.PushedRepos) > 0 {
					fmt.Fprintf(&activeLog, "job pushed to repos: %v\n", activeScriptResponse.PushedRepos)
				}
			}
		}

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
					ID:       active.ID,
					State:    active.State,
					Log:      activeLog.String(),
					Response: activeScriptResponse,
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
