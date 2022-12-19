package api

import (
	"time"

	"github.com/gofrs/uuid"
)

type Runner struct {
	ID, Arch                 string
	RegisteredAt, LastSeenAt time.Time
}

func (r Runner) Equal(other Runner) bool {
	return r.ID == other.ID && r.Arch == other.Arch
}

type JobState string

const (
	JobStateStarting JobState = "starting"
	JobStateRunning  JobState = "running"
	JobStateFinished JobState = "finished"
	JobStateErrored  JobState = "errored"
)

type JobPayload struct {
	Cmd []string
}

type JobID string

func (job JobID) LogID() string {
	return "job/" + string(job)
}

type Job struct {
	ID                               JobID
	State                            JobState
	Title                            string
	TargetRunnerID, TargetRunnerArch string
	Payload                          JobPayload
	Updated, Created                 time.Time
}

func NewJobID() (JobID, error) {
	uuid, err := uuid.NewV6()
	if err != nil {
		return "", err
	}
	return JobID(uuid.String()), nil
}
