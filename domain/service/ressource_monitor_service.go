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

// ResourceMonitorServiceImpl implémente le service de monitoring des ressources
type ResourceMonitorServiceImpl struct {
	domainRepo      outbound.DomainRepository
	queueService    inbound.QueueService
	statsHistory    []*inbound.ResourceStats
	lastStats       *inbound.ResourceStats
	maxHistorySize  int
	collectInterval time.Duration
	stopCollect     chan struct{}
	rootCtx         context.Context
	mu              sync.RWMutex
}

// NewResourceMonitorService crée un nouveau service de surveillance des ressources
func NewResourceMonitorService(
	domainRepo outbound.DomainRepository,
	queueService inbound.QueueService,
	rootCtx context.Context,
) inbound.ResourceMonitorService {
	log.Println("Initializing resource monitoring service")

	svc := &ResourceMonitorServiceImpl{
		domainRepo:      domainRepo,
		queueService:    queueService,
		statsHistory:    make([]*inbound.ResourceStats, 0, 60), // 1 heure d'historique à 1 point/minute
		maxHistorySize:  60,
		collectInterval: 1 * time.Minute,
		stopCollect:     make(chan struct{}),
		rootCtx:         rootCtx,
	}

	// Démarrer la collecte périodique
	go svc.startCollection(rootCtx)

	return svc
}

// startCollection démarre la collecte périodique des statistiques
func (s *ResourceMonitorServiceImpl) startCollection(ctx context.Context) {
	ticker := time.NewTicker(s.collectInterval)
	defer ticker.Stop()

	// Collecter immédiatement au démarrage
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

// collectStats collecte les statistiques sur l'utilisation des ressources
func (s *ResourceMonitorServiceImpl) collectStats(ctx context.Context) {
	// Obtenir les statistiques mémoire
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Statistiques générales
	stats := &inbound.ResourceStats{
		Timestamp:   time.Now().Unix(),
		MemoryUsage: int64(memStats.Alloc),
		Goroutines:  runtime.NumGoroutine(),
		GCCycles:    memStats.NumGC,
		GCPauseNs:   int64(memStats.PauseNs[(memStats.NumGC+255)%256]), // Dernière pause GC
		HeapObjects: memStats.HeapObjects,
		DomainStats: make(map[string]inbound.DomainResourceInfo),
	}

	// Collecter les statistiques par domaine
	domains, err := s.domainRepo.ListDomains(ctx)
	if err == nil {
		for _, domain := range domains {
			domainInfo := inbound.DomainResourceInfo{
				QueueCount:      len(domain.Queues),
				MessageCount:    0,
				QueueStats:      make(map[string]inbound.QueueResourceInfo),
				EstimatedMemory: 0,
			}

			// Collecte par queue
			for queueName, queue := range domain.Queues {
				queueInfo := inbound.QueueResourceInfo{
					MessageCount: queue.MessageCount,
					BufferSize:   queue.Config.MaxSize,
					// Estimation grossière: 1KB par message en moyenne
					// (ajustez selon la taille typique de vos messages)
					EstimatedMemory: int64(queue.MessageCount * 1024),
				}

				domainInfo.MessageCount += queue.MessageCount
				domainInfo.EstimatedMemory += queueInfo.EstimatedMemory
				domainInfo.QueueStats[queueName] = queueInfo
			}

			stats.DomainStats[domain.Name] = domainInfo
		}
	}

	// Stocker les statistiques
	s.mu.Lock()
	s.lastStats = stats
	s.statsHistory = append(s.statsHistory, stats)

	// Limiter la taille de l'historique
	if len(s.statsHistory) > s.maxHistorySize {
		s.statsHistory = s.statsHistory[len(s.statsHistory)-s.maxHistorySize:]
	}
	s.mu.Unlock()
}

// GetCurrentStats renvoie les statistiques actuelles
func (s *ResourceMonitorServiceImpl) GetCurrentStats(ctx context.Context) (*inbound.ResourceStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Si aucune statistique n'est disponible, collecter maintenant
	if s.lastStats == nil {
		s.mu.RUnlock() // Libérer le verrou en lecture avant de le reprendre en écriture
		s.collectStats(ctx)
		s.mu.RLock()
	}

	return s.lastStats, nil
}

// GetStatsHistory renvoie l'historique des statistiques
func (s *ResourceMonitorServiceImpl) GetStatsHistory(ctx context.Context, limit int) ([]*inbound.ResourceStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Créer une copie pour éviter les modifications externes
	result := make([]*inbound.ResourceStats, len(s.statsHistory))
	copy(result, s.statsHistory)

	// Appliquer la limite si nécessaire
	if limit > 0 && limit < len(result) {
		result = result[len(result)-limit:]
	}

	return result, nil
}

// Cleanup arrête proprement le service
func (s *ResourceMonitorServiceImpl) Cleanup() {
	log.Println("Cleaning up resource monitoring service")

	// Signaler l'arrêt de la collecte
	close(s.stopCollect)

	// Attendre un peu pour laisser le temps aux goroutines de s'arrêter
	time.Sleep(100 * time.Millisecond)

	s.mu.Lock()
	s.statsHistory = nil
	s.lastStats = nil
	s.mu.Unlock()

	log.Println("Resource monitoring service shutdown complete")
}
