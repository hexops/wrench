package wrench

import (
	"context"
	"runtime"
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
		started := false
		for {
			if started {
				time.Sleep(5 * time.Second)
			}
			started = true
			ctx := context.Background()
			resp, err := b.runner.RunnerPoll(ctx, &api.RunnerPollRequest{ID: b.Config.Runner, Arch: arch})
			if err != nil {
				b.logf("runner: error: %v", err)
				continue
			}
			if !connected {
				connected = true
				b.logf("runner: working for %s ('%s', %s)", b.Config.ExternalURL, b.Config.Runner, arch)
			}
			_ = resp
			b.logf("runner: waiting for jobs")
		}
	}()
	return nil
}
