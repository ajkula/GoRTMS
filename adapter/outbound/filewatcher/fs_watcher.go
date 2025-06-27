package filewatcher

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

type FsWatcher struct {
	watcher     *fsnotify.Watcher
	events      chan outbound.FileChangeEvent
	errors      chan error
	watchedDirs map[string]bool
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	running     bool
	closed      chan struct{}
}

func NewFSWatcher() (outbound.FileWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	fw := &FsWatcher{
		watcher:     fsWatcher,
		events:      make(chan outbound.FileChangeEvent, 100),
		errors:      make(chan error, 10),
		watchedDirs: make(map[string]bool),
		ctx:         ctx,
		cancel:      cancel,
		running:     false,
		closed:      make(chan struct{}),
	}

	go fw.processEvents()

	return fw, nil
}

func (fw *FsWatcher) Watch(ctx context.Context, path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}

	// for file paths, we need to watch the directory and filter events
	dir := filepath.Dir(absPath)

	// check if we're already watching this directory
	if fw.watchedDirs[dir] {
		return nil
	}

	// add directory to fsnotify watcher
	if err := fw.watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", dir, err)
	}

	fw.watchedDirs[dir] = true
	fw.running = true

	return nil
}

func (fw *FsWatcher) Stop() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if !fw.running {
		return nil
	}

	// cancel context to stop processing
	fw.cancel()

	// close fsnotify watcher
	if err := fw.watcher.Close(); err != nil {
		return fmt.Errorf("failed to close fsnotify watcher: %w", err)
	}

	// wait for goroutine to finish
	<-fw.closed

	close(fw.events)
	close(fw.errors)

	fw.running = false
	return nil
}

func (fw *FsWatcher) Events() <-chan outbound.FileChangeEvent {
	return fw.events
}

func (fw *FsWatcher) Errors() <-chan error {
	return fw.errors
}

func (fw *FsWatcher) IsWatching() bool {
	fw.mu.RLock()
	defer fw.mu.RUnlock()
	return fw.running
}

func (fw *FsWatcher) GetWatchedPaths() []string {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	paths := make([]string, 0, len(fw.watchedDirs))
	for path := range fw.watchedDirs {
		paths = append(paths, path)
	}
	return paths
}

// handles fsnotify events and converts them to our event format
func (fw *FsWatcher) processEvents() {
	defer close(fw.closed)

	for {
		select {
		case <-fw.ctx.Done():
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// convert fsnotify event to our event format
			changeEvent := fw.convertEvent(event)
			if changeEvent != nil {
				select {
				case fw.events <- *changeEvent:
				case <-fw.ctx.Done():
					return
				}
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}

			select {
			case fw.errors <- err:
			case <-fw.ctx.Done():
				return
			}
		}
	}
}

// converts fsnotify.Event to our FileChangeEvent
func (fw *FsWatcher) convertEvent(event fsnotify.Event) *outbound.FileChangeEvent {
	var eventType string

	// Map fsnotify operations to our event types
	switch {
	case event.Has(fsnotify.Create):
		eventType = "create"
	case event.Has(fsnotify.Write):
		eventType = "modify"
	case event.Has(fsnotify.Remove):
		eventType = "delete"
	case event.Has(fsnotify.Rename):
		eventType = "delete" // for simplicity
	case event.Has(fsnotify.Chmod):
		// ignore chmod events as they're usually not relevant for our use case
		return nil
	default:
		// unknown
		return nil
	}

	return &outbound.FileChangeEvent{
		FilePath:  event.Name,
		EventType: eventType,
	}
}
