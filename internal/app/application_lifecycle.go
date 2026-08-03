package app

import (
	"context"
	"sync"
)

// applicationLifecycleState serializes process lifecycle transitions without
// turning Application back into a server-shaped dependency container.
type applicationLifecycleState struct {
	mu                sync.Mutex
	startDone         chan struct{}
	startCancel       context.CancelFunc
	startErr          error
	started           int
	shutdownRequested bool
	shutdownDone      chan struct{}
	shutdownErr       error
}
