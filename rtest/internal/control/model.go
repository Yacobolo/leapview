package control

import (
	"errors"
	"time"

	"github.com/flidai/leapview/rtest/internal/protocol"
)

var (
	ErrAlreadyExists   = errors.New("already exists")
	ErrForbidden       = errors.New("forbidden")
	ErrNotFound        = errors.New("not found")
	ErrUnauthenticated = errors.New("unauthenticated")
)

type PrincipalKind string

const (
	PrincipalDevice PrincipalKind = "device"
	PrincipalGitHub PrincipalKind = "github"
)

type Principal struct {
	Kind      PrincipalKind
	TokenID   string
	UserID    string
	ProjectID string
	Admin     bool
	Subject   string
}

type User struct {
	ID        string
	Name      string
	Admin     bool
	CreatedAt time.Time
}

type Project struct {
	ID        string
	Slug      string
	Name      string
	CreatedAt time.Time
}

type Bootstrap struct {
	UserName    string
	ProjectSlug string
	ProjectName string
	TokenName   string
}

type BootstrapResult struct {
	User    User
	Project Project
	Token   string
}

type DeviceToken struct {
	ID         string
	Name       string
	UserID     string
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

type CreateDeviceToken struct {
	UserID    string
	Name      string
	ExpiresAt time.Time
}

type IssuedDeviceToken struct {
	Metadata DeviceToken
	Secret   string
}

type GitHubTrust struct {
	ID                string
	ProjectID         string
	RepositoryOwnerID string
	RepositoryID      string
	WorkflowRef       string
	Ref               string
	Environment       string
	Events            []string
	CreatedAt         time.Time
	RevokedAt         *time.Time
}

type GitHubClaims struct {
	Subject           string
	RepositoryOwnerID string
	RepositoryID      string
	Repository        string
	WorkflowRef       string
	Ref               string
	Environment       string
	EventName         string
	ExpiresAt         time.Time
}

type IssuedAccessToken struct {
	Token     string
	ExpiresAt time.Time
}

type Job struct {
	ID               string
	ProjectID        string
	Image            string
	Command          []string
	WorkingDirectory string
	Environment      map[string]string
	RootDigest       string
	Status           protocol.Status
	Timeout          time.Duration
	CPUs             string
	Memory           string
	CreatedAt        time.Time
	StartedAt        *time.Time
	FinishedAt       *time.Time
	ExitCode         *int
	ErrorMessage     string
	CancelRequested  bool
	WorkerID         string
}

type PrepareJob struct {
	ProjectID        string
	Image            string
	Command          []string
	WorkingDirectory string
	Environment      map[string]string
	Timeout          time.Duration
	CPUs             string
	Memory           string
}

type BuildStatus string

const (
	BuildRunning   BuildStatus = "running"
	BuildSucceeded BuildStatus = "succeeded"
	BuildFailed    BuildStatus = "failed"
	BuildCancelled BuildStatus = "cancelled"
)

type Build struct {
	ID         string
	ProjectID  string
	Status     BuildStatus
	CreatedAt  time.Time
	FinishedAt *time.Time
	ExitCode   *int
}
