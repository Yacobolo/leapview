package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type cleanupEntry struct {
	name  string
	close func(context.Context) error
}

type cleanupStack struct {
	mu      sync.Mutex
	entries []cleanupEntry
	once    sync.Once
	err     error
}

func (s *cleanupStack) Push(name string, close func(context.Context) error) {
	if close == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, cleanupEntry{name: name, close: close})
}

func (s *cleanupStack) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.once.Do(func() {
		s.mu.Lock()
		entries := append([]cleanupEntry(nil), s.entries...)
		s.entries = nil
		s.mu.Unlock()
		errs := make([]error, 0, len(entries))
		for index := len(entries) - 1; index >= 0; index-- {
			if err := entries[index].close(ctx); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", entries[index].name, err))
			}
		}
		s.err = errors.Join(errs...)
	})
	return s.err
}
