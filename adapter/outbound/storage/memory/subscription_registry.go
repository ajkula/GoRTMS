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

type Subscription struct {
	ID         string
	DomainName string
	QueueName  string
	Handler    model.MessageHandler
}

type SubscriptionRegistry struct {
	// Map of subscriptions by ID
	subscriptions map[string]*Subscription

	// Map of subscriptions by queue
	queueSubscriptions map[string]map[string]*Subscription

	mu sync.RWMutex
}

func NewSubscriptionRegistry() outbound.SubscriptionRegistry {
	return &SubscriptionRegistry{
		subscriptions:      make(map[string]*Subscription),
		queueSubscriptions: make(map[string]map[string]*Subscription),
	}
}

func (r *SubscriptionRegistry) RegisterSubscription(
	domainName, queueName string,
	handler model.MessageHandler,
) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate a unique ID
	id := generateSubscriptionID()

	// Create the subscription
	subscription := &Subscription{
		ID:         id,
		DomainName: domainName,
		QueueName:  queueName,
		Handler:    handler,
	}

	// Store the subscription
	r.subscriptions[id] = subscription

	// Create the queue key
	queueKey := fmt.Sprintf("%s:%s", domainName, queueName)

	// Create the map if necessary
	if _, exists := r.queueSubscriptions[queueKey]; !exists {
		r.queueSubscriptions[queueKey] = make(map[string]*Subscription)
	}

	// Associate the subscription with the queue
	r.queueSubscriptions[queueKey][id] = subscription

	return id, nil
}

func (r *SubscriptionRegistry) UnregisterSubscription(
	subscriptionID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Retrieve the subscription
	subscription, exists := r.subscriptions[subscriptionID]
	if !exists {
		return ErrSubscriptionNotFound
	}

	// Remove the subscription
	delete(r.subscriptions, subscriptionID)

	// Remove from the queue map
	queueKey := fmt.Sprintf("%s:%s", subscription.DomainName, subscription.QueueName)
	if queueSubs, exists := r.queueSubscriptions[queueKey]; exists {
		delete(queueSubs, subscriptionID)

		// Delete the queue map if it's empty
		if len(queueSubs) == 0 {
			delete(r.queueSubscriptions, queueKey)
		}
	}

	return nil
}

func (r *SubscriptionRegistry) NotifySubscribers(
	domainName, queueName string,
	message *model.Message,
) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create the queue key
	queueKey := fmt.Sprintf("%s:%s", domainName, queueName)

	// Retrieve the subscriptions
	queueSubs, exists := r.queueSubscriptions[queueKey]
	if !exists {
		return nil // No subs
	}

	// Notify each subscriber in a goroutine
	var wg sync.WaitGroup
	for _, subscription := range queueSubs {
		wg.Add(1)
		go func(sub *Subscription, msg *model.Message) {
			defer wg.Done()

			// Clone the message to avoid concurrency issues
			messageCopy := *msg

			// Call the handler
			if err := sub.Handler(&messageCopy); err != nil {
				// Log the error but continue
				fmt.Printf("Error notifying subscriber %s: %v\n", sub.ID, err)
			}
		}(subscription, message)
	}

	// Wait for all notifications to finish
	// Note: In a production system, we might use a timeout
	wg.Wait()

	return nil
}

func generateSubscriptionID() string {
	// Generate a random ID
	return fmt.Sprintf("sub-%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}
