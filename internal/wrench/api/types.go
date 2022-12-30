package api

import (
	"time"

	"github.com/google/go-github/v48/github"
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

	// Pushed is a list of repositories that were pushed to, if any.
	Pushed []string
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
	Secrets            map[string]string
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
	PRTemplate        PRTemplate
	Background        bool
	Cmd               []string
	SecretIDs         []string
	Ping              bool
}

type PRTemplate struct {
	Title, Body string
	Head, Base  string
	Draft       bool
}

func (pr *PRTemplate) ToGitHub() *github.NewPullRequest {
	return &github.NewPullRequest{
		Title: &pr.Title,
		Head:  &pr.Head,
		Base:  &pr.Base,
		Body:  &pr.Body,
		Draft: &pr.Draft,
	}
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
	ScheduledStart, Updated, Created time.Time
}
