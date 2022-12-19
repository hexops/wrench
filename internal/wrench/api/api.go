package api

type RunnerPollRequest struct {
	// ID is the unique identifier for this runner. It must not conflict with other runners.
	ID string

	// Arch is the architecture of this runner, in `$GOOS/$GOARCH` format.
	Arch string

	// Job, if non-nil, indicates the runner has an update about performing a job.
	Job *RunnerJobUpdate
}

type RunnerPollResponse struct {
	// Start, if non-nil, indicates the runner should start working on this job.
	Start *RunnerJobStart
}

type RunnerListRequest struct{}

type RunnerListResponse struct {
	Runners []Runner
}
