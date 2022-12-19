package api

type RunnerPollRequest struct {
	// ID is the unique identifier for this runner. It must not conflict with other runners.
	ID string

	// Arch is the architecture of this runner, in `$GOOS/$GOARCH` format.
	Arch string
}

type RunnerPollResponse struct{}

type RunnerListRequest struct{}

type RunnerListResponse struct {
	Runners []Runner
}
