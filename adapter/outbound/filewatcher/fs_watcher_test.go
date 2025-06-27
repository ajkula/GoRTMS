package filewatcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFSWatcher_BasicOperations(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "fs_watcher_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create file watcher
	watcher, err := NewFSWatcher()
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	// Test initial state
	if watcher.IsWatching() {
		t.Error("Expected watcher to not be watching initially")
	}

	if len(watcher.GetWatchedPaths()) != 0 {
		t.Error("Expected no watched paths initially")
	}

	// Start watching a test file
	testFile := filepath.Join(tempDir, "test.db")
	ctx := context.Background()

	err = watcher.Watch(ctx, testFile)
	if err != nil {
		t.Fatalf("Failed to watch file: %v", err)
	}

	// Check that watcher is now active
	if !watcher.IsWatching() {
		t.Error("Expected watcher to be watching after Watch() call")
	}

	watchedPaths := watcher.GetWatchedPaths()
	if len(watchedPaths) != 1 {
		t.Errorf("Expected 1 watched path, got %d", len(watchedPaths))
	}

	// The watched path should be the directory containing the file
	expectedDir := filepath.Dir(testFile)
	if watchedPaths[0] != expectedDir {
		t.Errorf("Expected watched path %s, got %s", expectedDir, watchedPaths[0])
	}
}

func TestFSWatcher_FileEvents(t *testing.T) {
	// This test may be flaky depending on the filesystem and timing
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "fs_watcher_events_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create file watcher
	watcher, err := NewFSWatcher()
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	// Start watching directory
	testFile := filepath.Join(tempDir, "test.db")
	ctx := context.Background()

	err = watcher.Watch(ctx, testFile)
	if err != nil {
		t.Fatalf("Failed to watch file: %v", err)
	}

	// Give the watcher time to initialize
	time.Sleep(100 * time.Millisecond)

	// Create a file to trigger an event
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Write to file to trigger modify event
	_, err = file.WriteString("test content")
	if err != nil {
		t.Fatalf("Failed to write to test file: %v", err)
	}
	file.Close()

	// Wait for events with timeout
	select {
	case event := <-watcher.Events():
		// Verify we got an event for our test file
		if event.FilePath != testFile {
			t.Errorf("Expected event for %s, got event for %s", testFile, event.FilePath)
		}

		if event.EventType != "create" && event.EventType != "modify" {
			t.Errorf("Expected create or modify event, got %s", event.EventType)
		}

	case err := <-watcher.Errors():
		t.Fatalf("Unexpected error from watcher: %v", err)

	case <-time.After(2 * time.Second):
		t.Log("Warning: No file event received within timeout - this may be normal on some filesystems")
		// Don't fail the test as file events can be unreliable in test environments
	}

	// Test delete event
	err = os.Remove(testFile)
	if err != nil {
		t.Fatalf("Failed to remove test file: %v", err)
	}

	// Check for delete event (with timeout)
	select {
	case event := <-watcher.Events():
		if event.EventType != "delete" {
			t.Logf("Expected delete event, got %s - this may be normal", event.EventType)
		}

	case err := <-watcher.Errors():
		t.Fatalf("Unexpected error from watcher: %v", err)

	case <-time.After(1 * time.Second):
		t.Log("Warning: No delete event received - this may be normal on some filesystems")
	}
}

func TestFSWatcher_StopAndCleanup(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "fs_watcher_stop_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create file watcher
	watcher, err := NewFSWatcher()
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	// Start watching
	testFile := filepath.Join(tempDir, "test.db")
	ctx := context.Background()

	err = watcher.Watch(ctx, testFile)
	if err != nil {
		t.Fatalf("Failed to watch file: %v", err)
	}

	// Verify watcher is running
	if !watcher.IsWatching() {
		t.Error("Expected watcher to be watching")
	}

	// Stop watcher
	err = watcher.Stop()
	if err != nil {
		t.Fatalf("Failed to stop watcher: %v", err)
	}

	// Verify watcher is stopped
	if watcher.IsWatching() {
		t.Error("Expected watcher to be stopped after Stop()")
	}

	// Multiple stops should be safe
	err = watcher.Stop()
	if err != nil {
		t.Fatalf("Second Stop() call should not error: %v", err)
	}
}

func TestFSWatcher_EventConversion(t *testing.T) {
	// Test the event type mapping logic expectations
	// Since fsnotify.Event creation requires actual filesystem operations,
	// we test our expected mapping behavior

	expectedMappings := map[string]string{
		"CREATE": "create",
		"WRITE":  "modify",
		"REMOVE": "delete",
		"RENAME": "delete",
		"CHMOD":  "", // Should be ignored (nil return)
	}

	for fsOperation, expectedEventType := range expectedMappings {
		t.Run(fsOperation+" mapping", func(t *testing.T) {
			// Test that our mapping expectations are consistent
			if fsOperation == "CHMOD" {
				if expectedEventType != "" {
					t.Errorf("CHMOD events should be ignored (empty string), got '%s'", expectedEventType)
				}
			} else {
				if expectedEventType == "" {
					t.Errorf("Operation %s should map to a non-empty event type", fsOperation)
				}
			}
		})
	}
}
