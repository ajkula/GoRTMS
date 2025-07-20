package filewatcher

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

type FsWatcher struct {
	watcher     *fsnotify.Watcher
	events      chan outbound.FileChangeEvent
	errors      chan error
	writeEvents chan fsnotify.Event
	debouncer   map[string]*time.Timer
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
		events:      make(chan outbound.FileChangeEvent, 1000),
		errors:      make(chan error, 100),
		writeEvents: make(chan fsnotify.Event, 100),
		debouncer:   make(map[string]*time.Timer),
		watchedDirs: make(map[string]bool),
		ctx:         ctx,
		cancel:      cancel,
		running:     false,
		closed:      make(chan struct{}),
	}

	go fw.filterToWriteEvents()
	go fw.processWriteEvents()

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

	if !fw.running {
		return nil
	}

	// cancel context to stop processing
	fw.cancel()

	// cleanup all debounce timers
	fw.cleanupDebouncers()

	// close fsnotify watcher
	if err := fw.watcher.Close(); err != nil {
		return fmt.Errorf("failed to close fsnotify watcher: %w", err)
	}

	fw.running = false
	fw.mu.Unlock()

	// wait for goroutines to finish
	<-fw.closed

	close(fw.events)
	close(fw.errors)
	close(fw.writeEvents)

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

// filterToWriteEvents filters fsnotify events to only Write/Create and applies debouncing
func (fw *FsWatcher) filterToWriteEvents() {
	for {
		select {
		case <-fw.ctx.Done():
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// Only process Write and Create events
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				fw.debounceEvent(event)
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

// processWriteEvents handles debounced write events with safety timeouts
func (fw *FsWatcher) processWriteEvents() {
	defer close(fw.closed)

	ticker := time.NewTicker(10 * time.Second) // Safety heartbeat
	defer ticker.Stop()

	for {
		select {
		case <-fw.ctx.Done():
			fw.mu.Lock()
			fw.cleanupDebouncers()
			fw.mu.Unlock()
			return

		case event := <-fw.writeEvents:
			changeEvent := fw.convertEvent(event)
			if changeEvent != nil {
				select {
				case fw.events <- *changeEvent:
				case <-fw.ctx.Done():
					return
				}
			}

		case <-ticker.C:
			// Periodic cleanup to prevent timer leaks
			fw.cleanupExpiredDebouncers()
		}
	}
}

// debounceEvent applies debouncing logic per file
func (fw *FsWatcher) debounceEvent(event fsnotify.Event) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Stop existing timer for this file
	if timer, exists := fw.debouncer[event.Name]; exists {
		timer.Stop()
	}

	// Create new debounce timer
	fw.debouncer[event.Name] = time.AfterFunc(2*time.Second, func() {
		select {
		case fw.writeEvents <- event:
		case <-fw.ctx.Done():
		}

		// Clean up timer reference
		fw.mu.Lock()
		delete(fw.debouncer, event.Name)
		fw.mu.Unlock()
	})
}

// cleanupDebouncers stops and removes all debounce timers
func (fw *FsWatcher) cleanupDebouncers() {
	for _, timer := range fw.debouncer {
		timer.Stop()
	}
	fw.debouncer = make(map[string]*time.Timer)
}

// cleanupExpiredDebouncers prevents timer accumulation
func (fw *FsWatcher) cleanupExpiredDebouncers() {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Safety limit - if too many timers, clean them all
	if len(fw.debouncer) > 100 {
		fw.cleanupDebouncers()
	}
}

// convertEvent converts fsnotify.Event to our FileChangeEvent (simplified for Write/Create only)
func (fw *FsWatcher) convertEvent(event fsnotify.Event) *outbound.FileChangeEvent {
	var eventType string

	if event.Has(fsnotify.Create) {
		eventType = "create"
	} else if event.Has(fsnotify.Write) {
		eventType = "modify"
	} else {
		return nil // Should not happen given our filtering
	}

	return &outbound.FileChangeEvent{
		FilePath:  event.Name,
		EventType: eventType,
	}
}
