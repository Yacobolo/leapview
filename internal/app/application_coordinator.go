package app

import (
	"context"
	"sync"
)

// applicationCoordinator owns the mutable process-lifecycle state. Keeping it
// separate from Application leaves the process-facing surface small while the
// coordinator serializes concurrent Start and Shutdown calls.
type applicationCoordinator struct {
	mu          sync.Mutex
	state       applicationState
	done        chan struct{}
	startErr    error
	shutdownErr error
	cleanupErr  error
	started     []bool
	stopReq     bool
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	startCancel context.CancelFunc
	cleanupDone bool
}
