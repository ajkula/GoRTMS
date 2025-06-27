package service

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

type fileWatcherService struct {
	watcher               outbound.FileWatcher
	accountRequestService inbound.AccountRequestService
	logger                outbound.Logger
	watchedFiles          map[string]bool
	mu                    sync.RWMutex
	ctx                   context.Context
	cancel                context.CancelFunc
	running               bool
}

func NewFileWatcherService(
	watcher outbound.FileWatcher,
	accountRequestService inbound.AccountRequestService,
	logger outbound.Logger,
) *fileWatcherService {
	ctx, cancel := context.WithCancel(context.Background())

	return &fileWatcherService{
		watcher:               watcher,
		accountRequestService: accountRequestService,
		logger:                logger,
		watchedFiles:          make(map[string]bool),
		ctx:                   ctx,
		cancel:                cancel,
		running:               false,
	}
}

// begins watching files and processing events
func (s *fileWatcherService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.logger.Warn("File watcher service already running")
		return nil
	}

	s.logger.Info("Starting file watcher service")

	go s.processEvents()

	s.running = true
	s.logger.Info("File watcher service started successfully")
	return nil
}

// stops the file watcher service
func (s *fileWatcherService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Stopping file watcher service")

	// cancel context to stop event processing
	s.cancel()

	// stop underlying watcher
	if err := s.watcher.Stop(); err != nil {
		s.logger.Error("Error stopping file watcher", "error", err)
		return err
	}

	s.running = false
	s.logger.Info("File watcher service stopped")
	return nil
}

// starts watching the account request database file
func (s *fileWatcherService) WatchAccountRequestFile(ctx context.Context, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// path normalization
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		s.logger.Error("Failed to get absolute path", "path", filePath, "error", err)
		return err
	}

	// check if already watching
	if s.watchedFiles[absPath] {
		s.logger.Debug("Already watching file", "path", absPath)
		return nil
	}

	s.logger.Info("Adding file to watch list", "path", absPath)

	if err := s.watcher.Watch(ctx, absPath); err != nil {
		s.logger.Error("Failed to watch file", "path", absPath, "error", err)
		return err
	}

	s.watchedFiles[absPath] = true
	s.logger.Info("Successfully watching file", "path", absPath)
	return nil
}

// returns true if the service is actively watching files
func (s *fileWatcherService) IsWatching() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running && s.watcher.IsWatching()
}

// returns a list of files currently being watched
func (s *fileWatcherService) GetWatchedFiles() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files := make([]string, 0, len(s.watchedFiles))
	for file := range s.watchedFiles {
		files = append(files, file)
	}
	return files
}

// handles file system events in a loop
func (s *fileWatcherService) processEvents() {
	s.logger.Info("Starting file event processing loop")

	// prevent excessive processing
	rateLimiter := time.NewTicker(100 * time.Millisecond)
	defer rateLimiter.Stop()

	// track last sync time to avoid duplicate processing
	lastSyncTime := make(map[string]time.Time)

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("File event processing stopped")
			return

		case event := <-s.watcher.Events():
			<-rateLimiter.C

			s.logger.Debug("Received file event", "path", event.FilePath, "type", event.EventType)

			//  is it an account request file
			if s.isAccountRequestFile(event.FilePath) {
				s.handleAccountRequestFileEvent(event, lastSyncTime)
			}

		case err := <-s.watcher.Errors():
			s.logger.Error("File watcher error", "error", err)
			// noop
		}
	}
}

// checks if the given file path is an account request database file
func (s *fileWatcherService) isAccountRequestFile(filePath string) bool {
	// simply checks based on filename pattern
	fileName := filepath.Base(filePath)
	return fileName == "account_requests.db" ||
		filepath.Ext(fileName) == ".db" &&
			filepath.Dir(filePath) != ""
}

// processes file events for account request files
func (s *fileWatcherService) handleAccountRequestFileEvent(event outbound.FileChangeEvent, lastSyncTime map[string]time.Time) {
	now := time.Now()

	// don't process same file too frequently
	if lastSync, exists := lastSyncTime[event.FilePath]; exists {
		if now.Sub(lastSync) < 1*time.Second {
			s.logger.Debug("Skipping file event due to rate limiting", "path", event.FilePath)
			return
		}
	}

	s.logger.Info("Processing account request file event", "path", event.FilePath, "type", event.EventType)

	switch event.EventType {
	case "create", "modify":
		// file created or modified
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.accountRequestService.SyncPendingRequests(ctx); err != nil {
			s.logger.Error("Failed to sync pending requests after file event",
				"error", err, "path", event.FilePath, "type", event.EventType)
		} else {
			s.logger.Info("Successfully synced pending requests", "path", event.FilePath)
		}

	case "delete":
		s.logger.Warn("Account request file was deleted", "path", event.FilePath)
		// could implement recovery logic here
	default:
		s.logger.Debug("Ignoring file event type", "type", event.EventType, "path", event.FilePath)
	}

	// update last sync time
	lastSyncTime[event.FilePath] = now
}

func (s *fileWatcherService) Cleanup() {
	s.logger.Info("Cleaning up file watcher service")
	if err := s.Stop(); err != nil {
		s.logger.Error("Error during cleanup", "error", err)
	}
}
