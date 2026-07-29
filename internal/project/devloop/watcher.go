package devloop

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	projectcompiler "github.com/flidai/leapview/internal/project/compiler"
)

const defaultDebounce = 150 * time.Millisecond

// Update is one observable development-loop build attempt. Candidate contains
// the last valid remote candidate even when Err reports a new invalid edit.
type Update struct {
	Result Result
	Err    error
}

type Watcher struct {
	projectPath    string
	service        *Service
	debounce       time.Duration
	newSource      func() (watchSource, error)
	resolveSources func(string) ([]string, error)
}

func NewWatcher(projectPath string, service *Service) (*Watcher, error) {
	return newWatcher(projectPath, service, watcherOptions{
		debounce:       defaultDebounce,
		newSource:      newFSNotifySource,
		resolveSources: projectcompiler.SourceFiles,
	})
}

type watcherOptions struct {
	debounce       time.Duration
	newSource      func() (watchSource, error)
	resolveSources func(string) ([]string, error)
}

func newWatcher(projectPath string, service *Service, options watcherOptions) (*Watcher, error) {
	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, err
	}
	if service == nil || options.debounce <= 0 ||
		options.newSource == nil || options.resolveSources == nil {
		return nil, fmt.Errorf("project watcher requires service, debounce, source, and resolver")
	}
	return &Watcher{
		projectPath:    projectPath,
		service:        service,
		debounce:       options.debounce,
		newSource:      options.newSource,
		resolveSources: options.resolveSources,
	}, nil
}

// Run performs an initial reconcile, then serializes debounced changes until
// context cancellation. Invalid edits are reported while the Service preserves
// the last valid candidate.
func (watcher *Watcher) Run(ctx context.Context, report func(Update)) error {
	if watcher == nil {
		return fmt.Errorf("project watcher is not configured")
	}
	if report == nil {
		report = func(Update) {}
	}
	source, err := watcher.newSource()
	if err != nil {
		return fmt.Errorf("create project file watcher: %w", err)
	}
	defer source.Close()
	events := source.Events()
	sourceErrors := source.Errors()

	tracked := make(map[string]struct{})
	watchedDirectories := make(map[string]struct{})
	installSources := func(paths []string) error {
		next := make(map[string]struct{}, len(paths))
		for _, path := range paths {
			path, err = filepath.Abs(path)
			if err != nil {
				return err
			}
			next[filepath.Clean(path)] = struct{}{}
			directory := filepath.Dir(path)
			if _, exists := watchedDirectories[directory]; exists {
				continue
			}
			if err := source.Add(directory); err != nil {
				return fmt.Errorf("watch project source directory %q: %w", directory, err)
			}
			watchedDirectories[directory] = struct{}{}
		}
		tracked = next
		return nil
	}
	resolveAndInstall := func() error {
		paths, err := watcher.resolveSources(watcher.projectPath)
		if err != nil {
			return err
		}
		return installSources(paths)
	}
	if err := resolveAndInstall(); err != nil {
		// Keep the manifest repairable even when the initial project is invalid.
		if addErr := source.Add(filepath.Dir(watcher.projectPath)); addErr != nil {
			return fmt.Errorf("watch project manifest: %w", addErr)
		}
		watchedDirectories[filepath.Dir(watcher.projectPath)] = struct{}{}
		tracked[watcher.projectPath] = struct{}{}
	}

	var lastResult Result
	reconcile := func() {
		result, reconcileErr := watcher.service.Reconcile(ctx)
		lastResult = result
		report(Update{Result: result, Err: reconcileErr})
		if reconcileErr == nil {
			if refreshErr := resolveAndInstall(); refreshErr != nil {
				report(Update{Result: result, Err: refreshErr})
			}
		}
	}
	reconcile()

	var timer *time.Timer
	var timerC <-chan time.Time
	schedule := func() {
		if timer == nil {
			timer = time.NewTimer(watcher.debounce)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(watcher.debounce)
		}
		timerC = timer.C
	}
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, open := <-events:
			if !open {
				events = nil
				if sourceErrors == nil {
					return fmt.Errorf("project file watcher closed")
				}
				continue
			}
			eventPath, err := filepath.Abs(event.name)
			if err != nil {
				continue
			}
			eventPath = filepath.Clean(eventPath)
			_, relevant := tracked[eventPath]
			if !relevant {
				paths, resolveErr := watcher.resolveSources(watcher.projectPath)
				if resolveErr != nil {
					continue
				}
				next := make(map[string]struct{}, len(paths))
				for _, path := range paths {
					absolute, absoluteErr := filepath.Abs(path)
					if absoluteErr != nil {
						continue
					}
					next[filepath.Clean(absolute)] = struct{}{}
				}
				if _, relevant = next[eventPath]; !relevant {
					continue
				}
				if installErr := installSources(paths); installErr != nil {
					report(Update{Result: lastResult, Err: installErr})
					continue
				}
			}
			schedule()
		case watchErr, open := <-sourceErrors:
			if !open {
				sourceErrors = nil
				if events == nil {
					return fmt.Errorf("project file watcher closed")
				}
				continue
			}
			if watchErr != nil {
				return fmt.Errorf("project file watcher: %w", watchErr)
			}
		case <-timerC:
			timerC = nil
			reconcile()
		}
	}
}

type fileEvent struct{ name string }

type watchSource interface {
	Add(string) error
	Events() <-chan fileEvent
	Errors() <-chan error
	Close() error
}

type fsNotifySource struct {
	watcher *fsnotify.Watcher
	events  chan fileEvent
	errors  chan error
	done    chan struct{}
	once    sync.Once
}

func newFSNotifySource() (watchSource, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	source := &fsNotifySource{
		watcher: watcher,
		events:  make(chan fileEvent),
		errors:  make(chan error),
		done:    make(chan struct{}),
	}
	go source.forward()
	return source, nil
}

func (source *fsNotifySource) Add(path string) error    { return source.watcher.Add(path) }
func (source *fsNotifySource) Events() <-chan fileEvent { return source.events }
func (source *fsNotifySource) Errors() <-chan error     { return source.errors }
func (source *fsNotifySource) Close() error {
	var err error
	source.once.Do(func() {
		close(source.done)
		err = source.watcher.Close()
	})
	return err
}

func (source *fsNotifySource) forward() {
	defer close(source.events)
	defer close(source.errors)
	for {
		select {
		case <-source.done:
			return
		case event, open := <-source.watcher.Events:
			if !open {
				return
			}
			select {
			case source.events <- fileEvent{name: event.Name}:
			case <-source.done:
				return
			}
		case err, open := <-source.watcher.Errors:
			if !open {
				return
			}
			select {
			case source.errors <- err:
			case <-source.done:
				return
			}
		}
	}
}
