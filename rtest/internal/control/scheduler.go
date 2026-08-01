package control

import (
	"context"
	"io"

	"github.com/flidai/leapview/rtest/internal/protocol"
)

type Scheduler interface {
	Check(context.Context) error
	ValidateImage(context.Context, string) error
	Create(context.Context, Job) error
	Status(context.Context, string) (protocol.Job, error)
	Logs(context.Context, string, bool, io.Writer) error
	Cancel(context.Context, string) error
}
