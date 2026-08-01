package protocol

import "time"

type Status string

const (
	StatusPreparing Status = "preparing"
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusTimedOut  Status = "timed_out"
	StatusLost      Status = "lost"
)

func (s Status) Terminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusCancelled, StatusTimedOut, StatusLost:
		return true
	default:
		return false
	}
}

type CreateJob struct {
	ID           string
	Repository   string
	Suite        string
	Runner       string
	Command      []string
	SourceDigest string
	Timeout      time.Duration
}

type Job struct {
	ID              string     `json:"id"`
	Repository      string     `json:"repository"`
	Suite           string     `json:"suite"`
	Runner          string     `json:"runner"`
	Command         []string   `json:"command"`
	SourceDigest    string     `json:"source_digest"`
	Status          Status     `json:"status"`
	WorkerID        string     `json:"worker_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	LeaseExpiresAt  *time.Time `json:"lease_expires_at,omitempty"`
	TimeoutSeconds  int        `json:"timeout_seconds"`
	CancelRequested bool       `json:"cancel_requested"`
	ExitCode        *int       `json:"exit_code,omitempty"`
	ErrorMessage    string     `json:"error_message,omitempty"`
}

type SubmitManifest struct {
	Repository     string   `json:"repository"`
	Suite          string   `json:"suite"`
	Runner         string   `json:"runner"`
	Command        []string `json:"command"`
	SourceDigest   string   `json:"source_digest"`
	SourceSize     int64    `json:"source_size"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

type ClaimRequest struct {
	WorkerID string `json:"worker_id"`
}

type FinishRequest struct {
	Status       Status `json:"status"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type LogChunk struct {
	Data string `json:"data"`
}
