package api

import "time"

type RunnerPollRequest struct {
	// ID is the unique identifier for this runner. It must not conflict with other runners.
	ID string

	// Arch is the architecture of this runner, in `$GOOS/$GOARCH` format.
	Arch string
}

type RunnerPollResponse struct {
}

type Runner struct {
	ID, Arch                 string
	RegisteredAt, LastSeenAt time.Time
}

func (r Runner) Equal(other Runner) bool {
	return r.ID == other.ID && r.Arch == other.Arch
}

type RunnerListRequest struct{}

type RunnerListResponse struct {
	Runners []Runner
}
