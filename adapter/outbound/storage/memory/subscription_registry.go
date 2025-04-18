package memory

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

var (
	ErrSubscriptionNotFound = errors.New("subscription not found")
)

// Subscription représente un abonnement à une file d'attente
type Subscription struct {
	ID         string
	DomainName string
	QueueName  string
	Handler    model.MessageHandler
}

// SubscriptionRegistry implémente un registry d'abonnements en mémoire
type SubscriptionRegistry struct {
	// Map des abonnements par ID
	subscriptions map[string]*Subscription

	// Map des abonnements par file d'attente
	queueSubscriptions map[string]map[string]*Subscription

	mu sync.RWMutex
}

// NewSubscriptionRegistry crée un nouveau registry d'abonnements
func NewSubscriptionRegistry() outbound.SubscriptionRegistry {
	return &SubscriptionRegistry{
		subscriptions:      make(map[string]*Subscription),
		queueSubscriptions: make(map[string]map[string]*Subscription),
	}
}

// RegisterSubscription enregistre un nouvel abonnement
func (r *SubscriptionRegistry) RegisterSubscription(
	domainName, queueName string,
	handler model.MessageHandler,
) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Générer un ID unique
	id := generateSubscriptionID()

	// Créer l'abonnement
	subscription := &Subscription{
		ID:         id,
		DomainName: domainName,
		QueueName:  queueName,
		Handler:    handler,
	}

	// Stocker l'abonnement
	r.subscriptions[id] = subscription

	// Créer la clé de file d'attente
	queueKey := fmt.Sprintf("%s:%s", domainName, queueName)

	// Créer la map si nécessaire
	if _, exists := r.queueSubscriptions[queueKey]; !exists {
		r.queueSubscriptions[queueKey] = make(map[string]*Subscription)
	}

	// Associer l'abonnement à la file d'attente
	r.queueSubscriptions[queueKey][id] = subscription

	return id, nil
}

// UnregisterSubscription supprime un abonnement
func (r *SubscriptionRegistry) UnregisterSubscription(
	subscriptionID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Récupérer l'abonnement
	subscription, exists := r.subscriptions[subscriptionID]
	if !exists {
		return ErrSubscriptionNotFound
	}

	// Supprimer l'abonnement
	delete(r.subscriptions, subscriptionID)

	// Supprimer de la map des files d'attente
	queueKey := fmt.Sprintf("%s:%s", subscription.DomainName, subscription.QueueName)
	if queueSubs, exists := r.queueSubscriptions[queueKey]; exists {
		delete(queueSubs, subscriptionID)

		// Supprimer la map des files d'attente si vide
		if len(queueSubs) == 0 {
			delete(r.queueSubscriptions, queueKey)
		}
	}

	return nil
}

// NotifySubscribers notifie tous les abonnés d'un message
func (r *SubscriptionRegistry) NotifySubscribers(
	domainName, queueName string,
	message *model.Message,
) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Créer la clé de file d'attente
	queueKey := fmt.Sprintf("%s:%s", domainName, queueName)

	// Récupérer les abonnements
	queueSubs, exists := r.queueSubscriptions[queueKey]
	if !exists {
		return nil // Pas d'abonnés, c'est OK
	}

	// Notifier chaque abonné dans une goroutine
	var wg sync.WaitGroup
	for _, subscription := range queueSubs {
		wg.Add(1)
		go func(sub *Subscription, msg *model.Message) {
			defer wg.Done()

			// Clone le message pour éviter les problèmes de concurrence
			messageCopy := *msg

			// Appeler le handler
			if err := sub.Handler(&messageCopy); err != nil {
				// Log l'erreur mais continue
				fmt.Printf("Error notifying subscriber %s: %v\n", sub.ID, err)
			}
		}(subscription, message)
	}

	// Attendre que toutes les notifications soient terminées
	// Note: Dans un système de production, on pourrait utiliser un timeout
	wg.Wait()

	return nil
}

// generateSubscriptionID génère un ID unique pour un abonnement
func generateSubscriptionID() string {
	// Initialiser le générateur de nombres aléatoires
	rand.Seed(time.Now().UnixNano())

	// Générer un ID aléatoire
	return fmt.Sprintf("sub-%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}
