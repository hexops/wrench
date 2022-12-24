package api

import (
	"time"
)

type Runner struct {
	ID, Arch                 string
	Env                      RunnerEnv
	RegisteredAt, LastSeenAt time.Time
}

func (r Runner) Equal(other Runner) bool {
	return r.ID == other.ID && r.Arch == other.Arch
}

type RunnerJobUpdate struct {
	// If this runner is owning a job, it must be specified here.
	ID JobID

	// State, if non-empty, is the new state of the job.
	State JobState

	// Log, if non-empty, are messages to log about the job.
	Log string

	// Pushed, if true, indicates changes were pushed to Git.
	Pushed bool
}

type RunnerEnv struct {
	WrenchVersion     string
	WrenchCommitTitle string
	WrenchDate        string
	WrenchGoVersion   string
}

type RunnerJobStart struct {
	ID      JobID
	Title   string
	Payload JobPayload

	GitPushUsername    string
	GitPushPassword    string
	GitConfigUserName  string
	GitConfigUserEmail string
}

type JobState string

const (
	JobStateReady    JobState = "ready"
	JobStateStarting JobState = "starting"
	JobStateRunning  JobState = "running"
	JobStateSuccess  JobState = "success"
	JobStateError    JobState = "error"
)

type JobPayload struct {
	GitPushBranchName string
	Cmd               []string
	Ping              bool
}

type JobID string

func (job JobID) LogID() string {
	return "job-" + string(job)
}

type Job struct {
	ID                               JobID
	State                            JobState
	Title                            string
	TargetRunnerID, TargetRunnerArch string
	Payload                          JobPayload
	Updated, Created                 time.Time
}
