package service

import (
	"context"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

type ResourceMonitorServiceImpl struct {
	domainRepo      outbound.DomainRepository
	messageRepo     outbound.MessageRepository
	queueService    inbound.QueueService
	statsHistory    []*inbound.ResourceStats
	lastStats       *inbound.ResourceStats
	maxHistorySize  int
	collectInterval time.Duration
	stopCollect     chan struct{}
	rootCtx         context.Context
	mu              sync.RWMutex
}

func NewResourceMonitorService(
	domainRepo outbound.DomainRepository,
	messageRepo outbound.MessageRepository,
	queueService inbound.QueueService,
	rootCtx context.Context,
) inbound.ResourceMonitorService {
	log.Println("Initializing resource monitoring service")

	svc := &ResourceMonitorServiceImpl{
		domainRepo:      domainRepo,
		messageRepo:     messageRepo,
		queueService:    queueService,
		statsHistory:    make([]*inbound.ResourceStats, 0, 60), // 1 hour of history at 1 point per minute
		maxHistorySize:  60,
		collectInterval: 1 * time.Minute,
		stopCollect:     make(chan struct{}),
		rootCtx:         rootCtx,
	}

	// Start collecting
	go svc.startCollection(rootCtx)

	return svc
}

func (s *ResourceMonitorServiceImpl) startCollection(ctx context.Context) {
	ticker := time.NewTicker(s.collectInterval)
	defer ticker.Stop()

	s.collectStats(ctx)

	for {
		select {
		case <-ticker.C:
			s.collectStats(ctx)
		case <-s.stopCollect:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *ResourceMonitorServiceImpl) collectStats(ctx context.Context) {
	// Get memory statistics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// general
	stats := &inbound.ResourceStats{
		Timestamp:   time.Now().Unix(),
		MemoryUsage: int64(memStats.Alloc),
		Goroutines:  runtime.NumGoroutine(),
		GCCycles:    memStats.NumGC,
		GCPauseNs:   int64(memStats.PauseNs[(memStats.NumGC+255)%256]), // Last GC pause
		HeapObjects: memStats.HeapObjects,
		DomainStats: make(map[string]inbound.DomainResourceInfo),
	}

	// by domain
	domains, err := s.domainRepo.ListDomains(ctx)
	if err == nil {
		for _, domain := range domains {
			domainInfo := inbound.DomainResourceInfo{
				QueueCount:      len(domain.Queues),
				MessageCount:    0,
				QueueStats:      make(map[string]inbound.QueueResourceInfo),
				EstimatedMemory: 0,
			}

			// by queue
			for queueName, queue := range domain.Queues {
				queueInfo := inbound.QueueResourceInfo{
					MessageCount: queue.MessageCount,
					BufferSize:   queue.Config.MaxSize,
					// Rough estimate: 1KB per message on average
					// (adjust this according to the typical size of your messages)
					EstimatedMemory: int64(queue.MessageCount * 1024),
				}

				domainInfo.MessageCount += s.messageRepo.GetQueueMessageCount(domain.Name, queueName)
				domainInfo.EstimatedMemory += queueInfo.EstimatedMemory
				domainInfo.QueueStats[queueName] = queueInfo
			}

			stats.DomainStats[domain.Name] = domainInfo
		}
	}

	// Store the statistics
	s.mu.Lock()
	s.lastStats = stats
	s.statsHistory = append(s.statsHistory, stats)

	// Limit the size of the history
	if len(s.statsHistory) > s.maxHistorySize {
		s.statsHistory = s.statsHistory[len(s.statsHistory)-s.maxHistorySize:]
	}
	s.mu.Unlock()
}

func (s *ResourceMonitorServiceImpl) GetCurrentStats(ctx context.Context) (*inbound.ResourceStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// If no statistics are available, collect them now
	if s.lastStats == nil {
		s.mu.RUnlock() // Release the read lock before acquiring the write lock
		s.collectStats(ctx)
		s.mu.RLock()
	}

	return s.lastStats, nil
}

func (s *ResourceMonitorServiceImpl) GetStatsHistory(ctx context.Context, limit int) ([]*inbound.ResourceStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid external modifications
	result := make([]*inbound.ResourceStats, len(s.statsHistory))
	copy(result, s.statsHistory)

	// Apply the limit if necessary
	if limit > 0 && limit < len(result) {
		result = result[len(result)-limit:]
	}

	return result, nil
}

func (s *ResourceMonitorServiceImpl) Cleanup() {
	log.Println("Cleaning up resource monitoring service")

	// Signal the stop of the collection
	close(s.stopCollect)

	// Wait a bit to allow goroutines time to stop
	time.Sleep(100 * time.Millisecond)

	s.mu.Lock()
	s.statsHistory = nil
	s.lastStats = nil
	s.mu.Unlock()

	log.Println("Resource monitoring service shutdown complete")
}
