package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

// StatsData représente les statistiques du système
type StatsData struct {
	Domains       int            `json:"domains"`
	Queues        int            `json:"queues"`
	Messages      int            `json:"messages"`
	Routes        int            `json:"routes"`
	MessageRates  []MessageRate  `json:"messageRates"`
	ActiveDomains []DomainStats  `json:"activeDomains"`
	QueueAlerts   []QueueAlert   `json:"queueAlerts"`
	DomainTrend   *Trend         `json:"domainTrend"`
	QueueTrend    *Trend         `json:"queueTrend"`
	MessageTrend  *Trend         `json:"messageTrend"`
	RouteTrend    *Trend         `json:"routeTrend"`
	TopQueues     []QueueStats   `json:"topQueues"`
	PublishCounts map[string]int `json:"publishCounts"`
	ConsumeCounts map[string]int `json:"consumeCounts"`
}

// Trend représente une tendance avec une direction et une valeur
type Trend struct {
	Direction string  `json:"direction"` // "up" ou "down"
	Value     float64 `json:"value"`     // pourcentage
}

// MessageRate représente un taux de messages à un moment donné
type MessageRate struct {
	Timestamp      int64   `json:"timestamp"`
	Rate           float64 `json:"rate"`
	Published      float64 `json:"published"`
	Consumed       float64 `json:"consumed"`
	PublishedTotal int     `json:"publishedTotal"`
	ConsumedTotal  int     `json:"consumedTotal"`
}

// DomainStats représente les statistiques d'un domaine
type DomainStats struct {
	Name         string  `json:"name"`
	QueueCount   int     `json:"queueCount"`
	MessageCount int     `json:"messageCount"`
	MessageRate  float64 `json:"messageRate"`
}

// QueueStats représente les statistiques d'une file d'attente
type QueueStats struct {
	Domain       string  `json:"domain"`
	Name         string  `json:"name"`
	MessageCount int     `json:"messageCount"`
	MaxSize      int     `json:"maxSize"`
	Usage        float64 `json:"usage"` // pourcentage d'utilisation
}

// QueueAlert représente une alerte pour une file d'attente
type QueueAlert struct {
	Domain     string  `json:"domain"`
	Queue      string  `json:"queue"`
	Usage      float64 `json:"usage"`    // pourcentage d'utilisation
	Severity   string  `json:"severity"` // "warning", "critical"
	DetectedAt int64   `json:"detectedAt"`
}

// MetricsStore est un store de métriques pour le service de statistiques
type MetricsStore struct {
	// Historique des taux de messages
	messageRates []MessageRate

	// Compteurs de messages publiés par queue
	publishCounters map[string]map[string]int // domainName -> queueName -> count

	// Compteurs de messages consommés par queue
	consumeCounters map[string]map[string]int // domainName -> queueName -> count

	// Alertes actives sur les files d'attente
	queueAlerts map[string]map[string]QueueAlert // domainName -> queueName -> alert

	// État précédent pour calculer les tendances
	previousStats *StatsData

	// Horodatage de la dernière collecte
	lastCollected time.Time

	// Mutex pour les accès concurrents
	mu sync.RWMutex
}

// StatsServiceImpl implémente le service des statistiques
type StatsServiceImpl struct {
	domainRepo  outbound.DomainRepository
	messageRepo outbound.MessageRepository
	metrics     *MetricsStore

	// Intervalle de collecte des métriques
	collectInterval time.Duration

	// Canal pour arrêter la collecte automatique
	stopCollect chan struct{}
}

// NewStatsService crée un nouveau service de statistiques
func NewStatsService(
	domainRepo outbound.DomainRepository,
	messageRepo outbound.MessageRepository,
) inbound.StatsService {
	metrics := &MetricsStore{
		messageRates:    make([]MessageRate, 0, 24), // Garder 24 points de données
		publishCounters: make(map[string]map[string]int),
		consumeCounters: make(map[string]map[string]int),
		queueAlerts:     make(map[string]map[string]QueueAlert),
		lastCollected:   time.Now(),
	}

	service := &StatsServiceImpl{
		domainRepo:      domainRepo,
		messageRepo:     messageRepo,
		metrics:         metrics,
		collectInterval: 1 * time.Minute, // Collecter toutes les 5 minutes
		stopCollect:     make(chan struct{}),
	}

	// Démarrer la collecte automatique des métriques
	go service.startMetricsCollection()

	return service
}

// TrackMessagePublished enregistre un message publié dans les métriques
func (s *StatsServiceImpl) TrackMessagePublished(domainName, queueName string) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	// Initialiser la map si nécessaire
	if _, exists := s.metrics.publishCounters[domainName]; !exists {
		s.metrics.publishCounters[domainName] = make(map[string]int)
	}

	// Incrémenter le compteur
	s.metrics.publishCounters[domainName][queueName]++
}

// TrackMessageConsumed enregistre un message consommé dans les métriques
func (s *StatsServiceImpl) TrackMessageConsumed(domainName, queueName string) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	// Initialiser la map si nécessaire
	if _, exists := s.metrics.consumeCounters[domainName]; !exists {
		s.metrics.consumeCounters[domainName] = make(map[string]int)
	}

	// Incrémenter le compteur
	s.metrics.consumeCounters[domainName][queueName]++
}

// startMetricsCollection démarre la collecte périodique des métriques
func (s *StatsServiceImpl) startMetricsCollection() {
	ticker := time.NewTicker(s.collectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.collectMetrics()
		case <-s.stopCollect:
			return
		}
	}
}

// collectMetrics collecte les métriques du système
func (s *StatsServiceImpl) collectMetrics() {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(s.metrics.lastCollected).Seconds()

	// Agréger les compteurs
	totalPublished := 0
	totalConsumed := 0

	// Calculer les totaux
	for _, domainCounters := range s.metrics.publishCounters {
		for _, count := range domainCounters {
			totalPublished += count
		}
	}

	for _, domainCounters := range s.metrics.consumeCounters {
		for _, count := range domainCounters {
			totalConsumed += count
		}
	}

	// Calculer les taux par seconde
	publishRate := float64(totalPublished) / elapsed
	consumeRate := float64(totalConsumed) / elapsed
	totalRate := publishRate + consumeRate

	// Ajouter au tableau des taux, maintenant avec les totaux
	s.metrics.messageRates = append(s.metrics.messageRates, MessageRate{
		Timestamp:      now.Unix(),
		Rate:           totalRate,
		Published:      publishRate,
		Consumed:       consumeRate,
		PublishedTotal: totalPublished, // Ajouter le total des messages publiés
		ConsumedTotal:  totalConsumed,  // Ajouter le total des messages consommés
	})

	// Limiter la taille de l'historique (garder 24 derniers points)
	if len(s.metrics.messageRates) > 24 {
		s.metrics.messageRates = s.metrics.messageRates[len(s.metrics.messageRates)-24:]
	}

	// Réinitialiser les compteurs
	s.metrics.publishCounters = make(map[string]map[string]int)
	s.metrics.consumeCounters = make(map[string]map[string]int)

	// Mettre à jour l'horodatage
	s.metrics.lastCollected = now

	// Vérifier les alertes de files d'attente pleines
	go s.checkQueueAlerts()
}

// checkQueueAlerts vérifie les alertes sur les files d'attente
func (s *StatsServiceImpl) checkQueueAlerts() {
	ctx := context.Background()

	// Récupérer tous les domaines
	domains, err := s.domainRepo.ListDomains(ctx)
	if err != nil {
		log.Printf("Error fetching domains for alerts: %v", err)
		return
	}

	// Verrouiller les métriques pour la mise à jour
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	// Réinitialiser les alertes après 1 heure
	expireBefore := time.Now().Add(-1 * time.Hour).Unix()

	// Vérifier chaque domaine
	for _, domain := range domains {
		// Initialiser la map si nécessaire
		if _, exists := s.metrics.queueAlerts[domain.Name]; !exists {
			s.metrics.queueAlerts[domain.Name] = make(map[string]QueueAlert)
		}

		// Parcourir les files d'attente
		for queueName, queue := range domain.Queues {
			// Vérifier si maxSize est défini et > 0
			if queue.Config.MaxSize > 0 {
				usage := float64(queue.MessageCount) / float64(queue.Config.MaxSize) * 100

				// Vérifier si l'utilisation dépasse les seuils
				if usage >= 90 {
					// Alerte critique
					s.metrics.queueAlerts[domain.Name][queueName] = QueueAlert{
						Domain:     domain.Name,
						Queue:      queueName,
						Usage:      usage,
						Severity:   "critical",
						DetectedAt: time.Now().Unix(),
					}
				} else if usage >= 75 {
					// Alerte d'avertissement
					s.metrics.queueAlerts[domain.Name][queueName] = QueueAlert{
						Domain:     domain.Name,
						Queue:      queueName,
						Usage:      usage,
						Severity:   "warning",
						DetectedAt: time.Now().Unix(),
					}
				} else {
					// Supprimer l'alerte existante si l'utilisation est revenue à la normale
					delete(s.metrics.queueAlerts[domain.Name], queueName)
				}
			}
		}

		// Supprimer les alertes expirées
		for queueName, alert := range s.metrics.queueAlerts[domain.Name] {
			if alert.DetectedAt < expireBefore {
				delete(s.metrics.queueAlerts[domain.Name], queueName)
			}
		}

		// Supprimer la map si vide
		if len(s.metrics.queueAlerts[domain.Name]) == 0 {
			delete(s.metrics.queueAlerts, domain.Name)
		}
	}
}

// GetStats récupère les statistiques du système
func (s *StatsServiceImpl) GetStats(ctx context.Context) (interface{}, error) {
	log.Println("Getting system statistics")

	// Récupérer les domaines
	domains, err := s.domainRepo.ListDomains(ctx)
	if err != nil {
		return nil, err
	}

	// Récupérer l'état précédent pour calculer les tendances
	var previousStats *StatsData
	s.metrics.mu.RLock()
	previousStats = s.metrics.previousStats
	s.metrics.mu.RUnlock()

	// Calculer les statistiques
	stats := &StatsData{
		Domains:       len(domains),
		Queues:        0,
		Messages:      0,
		Routes:        0,
		ActiveDomains: make([]DomainStats, 0, len(domains)),
		TopQueues:     make([]QueueStats, 0, 5),
		PublishCounts: make(map[string]int),
		ConsumeCounts: make(map[string]int),
	}

	// Récupérer les taux de messages
	s.metrics.mu.RLock()
	stats.MessageRates = make([]MessageRate, len(s.metrics.messageRates))
	copy(stats.MessageRates, s.metrics.messageRates)

	// Récupérer les alertes
	queueAlerts := []QueueAlert{}
	for _, domainAlerts := range s.metrics.queueAlerts {
		for _, alert := range domainAlerts {
			queueAlerts = append(queueAlerts, alert)
		}
	}
	stats.QueueAlerts = queueAlerts
	s.metrics.mu.RUnlock()

	// Calculer les statistiques par domaine
	allQueues := make([]QueueStats, 0)

	for _, domain := range domains {
		queueCount := len(domain.Queues)
		messageCount := 0
		routeCount := 0

		// Compter les messages et les règles
		for queueName, queue := range domain.Queues {
			messageCount += queue.MessageCount

			// Ajouter à la liste de toutes les files d'attente pour trier les top files
			maxSize := queue.Config.MaxSize
			if maxSize <= 0 {
				maxSize = 1000 // Valeur par défaut
			}

			usage := float64(queue.MessageCount) / float64(maxSize) * 100
			allQueues = append(allQueues, QueueStats{
				Domain:       domain.Name,
				Name:         queueName,
				MessageCount: queue.MessageCount,
				MaxSize:      maxSize,
				Usage:        usage,
			})

			// Compter les règles de routage
			if routes, exists := domain.Routes[queueName]; exists {
				routeCount += len(routes)
			}
		}

		// Ajouter aux totaux
		stats.Queues += queueCount
		stats.Messages += messageCount
		stats.Routes += routeCount

		// Ajouter aux stats du domaine
		if queueCount > 0 {
			stats.ActiveDomains = append(stats.ActiveDomains, DomainStats{
				Name:         domain.Name,
				QueueCount:   queueCount,
				MessageCount: messageCount,
				MessageRate:  calculateDomainMessageRate(domain.Name, stats.MessageRates),
			})
		}
	}

	// Trier et limiter les top files d'attente
	sortQueuesByUsage(allQueues)
	if len(allQueues) > 5 {
		stats.TopQueues = allQueues[:5]
	} else {
		stats.TopQueues = allQueues
	}

	// Calculer les tendances si on a des stats précédentes
	if previousStats != nil {
		stats.DomainTrend = calculateTrend(previousStats.Domains, stats.Domains)
		stats.QueueTrend = calculateTrend(previousStats.Queues, stats.Queues)
		stats.MessageTrend = calculateTrend(previousStats.Messages, stats.Messages)
		stats.RouteTrend = calculateTrend(previousStats.Routes, stats.Routes)
	}

	// Stocker les statistiques actuelles pour la prochaine fois
	s.metrics.mu.Lock()
	s.metrics.previousStats = stats
	s.metrics.mu.Unlock()

	return stats, nil
}

// calculateDomainMessageRate calcule le taux de messages moyen pour un domaine
func calculateDomainMessageRate(domainName string, rates []MessageRate) float64 {
	if len(rates) == 0 {
		return 0
	}

	// Pour l'instant, retourner simplement le dernier taux global
	// Dans une implémentation complète, on pourrait avoir des taux par domaine
	return rates[len(rates)-1].Rate
}

// calculateTrend calcule la tendance entre deux valeurs
func calculateTrend(previous, current int) *Trend {
	if previous == 0 {
		return nil
	}

	change := float64(current-previous) / float64(previous) * 100
	direction := "up"
	if change < 0 {
		direction = "down"
		change = -change
	}

	return &Trend{
		Direction: direction,
		Value:     change,
	}
}

// sortQueuesByUsage trie les files d'attente par utilisation
func sortQueuesByUsage(queues []QueueStats) {
	// Tri simple par usage décroissant
	for i := 0; i < len(queues); i++ {
		for j := i + 1; j < len(queues); j++ {
			if queues[j].Usage > queues[i].Usage {
				queues[i], queues[j] = queues[j], queues[i]
			}
		}
	}
}
