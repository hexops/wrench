package api

type RunnerPollRequest struct {
	// ID is the unique identifier for this runner. It must not conflict with other runners.
	ID string

	// Arch is the architecture of this runner, in `$GOOS/$GOARCH` format.
	Arch string

	// Running is the list of running jobs.
	Running []JobID

	// General runner environment info (wrench version, etc.)
	Env RunnerEnv
}

type RunnerPollResponse struct {
	// Start, if non-nil, indicates the runner should start working on this job.
	Start *RunnerJobStart

	NotFound bool
}

type RunnerJobUpdateRequest struct {
	// ID is the unique identifier for this runner. It must not conflict with other runners.
	ID string

	// Arch is the architecture of this runner, in `$GOOS/$GOARCH` format.
	Arch string

	// Job, if non-nil, indicates the runner has an update about performing a job.
	Job *RunnerJobUpdate
}

type RunnerJobUpdateResponse struct {
	// Start, if non-nil, indicates the runner should start working on this job.
	Start *RunnerJobStart

	NotFound bool
}

type RunnerListRequest struct{}

type RunnerListResponse struct {
	Runners []Runner
}

type SecretsListRequest struct{}

type SecretsListResponse struct {
	IDs []string
}

type SecretsDeleteRequest struct {
	ID string
}

type SecretsDeleteResponse struct{}

type SecretsUpsertRequest struct {
	ID, Value string
}

type SecretsUpsertResponse struct{}
