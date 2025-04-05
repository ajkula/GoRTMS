package service

import (
	"context"
	"log"
	"time"

	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

// StatsData représente les statistiques du système
type StatsData struct {
	Domains       int           `json:"domains"`
	Queues        int           `json:"queues"`
	Messages      int           `json:"messages"`
	Routes        int           `json:"routes"`
	MessageRates  []MessageRate `json:"messageRates"`
	ActiveDomains []DomainStats `json:"activeDomains"`
}

// MessageRate représente un taux de messages à un moment donné
type MessageRate struct {
	Timestamp int64   `json:"timestamp"`
	Rate      float64 `json:"rate"`
}

// DomainStats représente les statistiques d'un domaine
type DomainStats struct {
	Name         string  `json:"name"`
	QueueCount   int     `json:"queueCount"`
	MessageCount int     `json:"messageCount"`
	MessageRate  float64 `json:"messageRate"`
}

// StatsServiceImpl implémente le service des statistiques
type StatsServiceImpl struct {
	domainRepo outbound.DomainRepository
}

// NewStatsService crée un nouveau service de statistiques
func NewStatsService(
	domainRepo outbound.DomainRepository,
) inbound.StatsService {
	return &StatsServiceImpl{
		domainRepo: domainRepo,
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

	// Calculer les statistiques
	stats := &StatsData{
		Domains:       len(domains),
		Queues:        0,
		Messages:      0,
		Routes:        0,
		MessageRates:  generateSampleRates(),
		ActiveDomains: make([]DomainStats, 0, len(domains)),
	}

	// Calculer les statistiques par domaine
	for _, domain := range domains {
		queueCount := len(domain.Queues)
		messageCount := 0
		routeCount := 0

		// Compter les messages et les règles
		for queueName, queue := range domain.Queues {
			messageCount += queue.MessageCount

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
				MessageRate:  1.0, // Simuler un taux pour l'exemple
			})
		}
	}

	return stats, nil
}

// generateSampleRates génère des exemples de taux de messages pour la démo
func generateSampleRates() []MessageRate {
	rates := make([]MessageRate, 10)
	now := time.Now().Unix()

	for i := 0; i < 10; i++ {
		rates[i] = MessageRate{
			Timestamp: now - int64((9-i)*3600), // Dernières 10 heures
			Rate:      float64(i + 5),          // Taux croissant pour la démo
		}
	}

	return rates
}
