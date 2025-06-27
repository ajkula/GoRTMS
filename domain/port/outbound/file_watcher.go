package outbound

import (
	"context"
)

// represents a file system change event
type FileChangeEvent struct {
	FilePath  string `json:"filePath"`  // Path to the changed file
	EventType string `json:"eventType"` // Type of event: "create", "modify", "delete"
}

// defines operations for monitoring file system changes
type FileWatcher interface {
	// starts monitoring a file or directory for changes
	Watch(ctx context.Context, path string) error

	// stops watching all files and releases resources
	Stop() error

	// returns a channel for receiving file change events
	Events() <-chan FileChangeEvent

	// returns a channel for receiving file watcher errors
	Errors() <-chan error

	// returns true if the watcher is currently monitoring files
	IsWatching() bool

	// returns a list of currently watched paths
	GetWatchedPaths() []string
}
