package model

import (
	"context"
	"errors"
	"log"
	"math"
	"sync"
	"time"
)

var (
	ErrQueueClosed = errors.New("queue is closed")
	ErrQueueFull   = errors.New("queue is full")
)

type ChannelQueue struct {
	queue        *Queue
	messages     chan *Message
	subscribers  []MessageHandler
	mu           sync.RWMutex
	workerCtx    context.Context
	workerCancel context.CancelFunc
	bufferSize   int

	wg        sync.WaitGroup //suivi des workers
	workerSem chan struct{}  // Sémaphore pour limiter les goroutines concurrentes

	// Gestion des erreurs
	retryQueue     chan *MessageWithRetry //File de msg à réessayer
	circuitBreaker *CircuitBreaker
}

// NewChannelQueue crée une nouvelle queue basée sur des channels
func NewChannelQueue(queue *Queue, ctx context.Context, bufferSize int) *ChannelQueue {
	if bufferSize <= 0 {
		bufferSize = 100 // default
	}

	workerCtx, cancel := context.WithCancel(ctx)

	// Déterminer le nombre de workers
	workerCount := queue.Config.WorkerCount
	if workerCount <= 0 {
		// Utiliser un nombre par défaut basé sur le mode de livraison
		if queue.Config.DeliveryMode == BroadcastMode {
			workerCount = 2
		} else {
			workerCount = 1
		}
	}

	// Créer le circuit breaker si activé
	var cb *CircuitBreaker
	if queue.Config.CircuitBreakerEnabled && queue.Config.CircuitBreakerConfig != nil {
		cb = &CircuitBreaker{
			ErrorThreshold:   queue.Config.CircuitBreakerConfig.ErrorThreshold,
			SuccessThreshold: queue.Config.CircuitBreakerConfig.SuccessThreshold,
			MinimumRequests:  queue.Config.CircuitBreakerConfig.MinimumRequests,
			OpenTimeout:      queue.Config.CircuitBreakerConfig.OpenTimeout,
			State:            CircuitClosed,
			LastStateChange:  time.Now(),
		}

		// Valeurs par défaut si non spécifiées
		if cb.ErrorThreshold <= 0 {
			cb.ErrorThreshold = 0.5 // 50% par défaut
		}
		if cb.SuccessThreshold <= 0 {
			cb.SuccessThreshold = 5
		}
		if cb.MinimumRequests <= 0 {
			cb.MinimumRequests = 10
		}
		if cb.OpenTimeout <= 0 {
			cb.OpenTimeout = 30 * time.Second
		}
	}

	// Créer une file de retry si activée
	var retryQueue chan *MessageWithRetry
	if queue.Config.RetryEnabled {
		retryQueue = make(chan *MessageWithRetry, bufferSize)
	}

	return &ChannelQueue{
		queue:          queue,
		messages:       make(chan *Message, bufferSize),
		subscribers:    make([]MessageHandler, 0),
		workerCtx:      workerCtx,
		workerCancel:   cancel,
		bufferSize:     bufferSize,
		wg:             sync.WaitGroup{},
		workerSem:      make(chan struct{}, workerCount),
		retryQueue:     retryQueue,
		circuitBreaker: cb,
	}
}

// GetQueue return la queue sous-jacente
func (cq *ChannelQueue) GetQueue() *Queue {
	return cq.queue
}

// Enqueue ajoute un message à la queue
func (cq *ChannelQueue) Enqueue(ctx context.Context, message *Message) error {
	// Vérifier l'état du circuit breaker si activé
	if cq.circuitBreaker != nil {
		cq.circuitBreaker.mu.RLock()
		state := cq.circuitBreaker.State
		cq.circuitBreaker.mu.RUnlock()

		if state == CircuitOpen {
			return errors.New("circuit breaker open, message rejected")
		}
	}

	select {
	case <-cq.workerCtx.Done():
		return ErrQueueClosed
	case <-ctx.Done():
		return ctx.Err()
	case cq.messages <- message:
		// Mettre à jour le compteur de messages
		cq.queue.MessageCount++

		// Enregistrer un succès également ici
		if cq.circuitBreaker != nil {
			cq.recordSuccessInCircuitBreaker()
		}
		return nil
	default:
		// Si taille max et pleine
		if cq.queue.Config.MaxSize > 0 && cq.queue.MessageCount >= cq.queue.Config.MaxSize {
			return ErrQueueFull
		}

		// Sinon, essayer d'ajouter de manière bloquante avec timeout
		timer := time.NewTimer(5000 * time.Millisecond)
		defer timer.Stop()

		select {
		case <-cq.workerCtx.Done():
			return ErrQueueClosed
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return ErrQueueFull
		case cq.messages <- message:
			cq.queue.MessageCount++

			// Enregistrer un succès
			if cq.circuitBreaker != nil {
				cq.recordSuccessInCircuitBreaker()
			}
			return nil
		}
	}
}

// Méthode helper pour factoriser le code
func (cq *ChannelQueue) recordSuccessInCircuitBreaker() {
	cq.circuitBreaker.mu.Lock()
	defer cq.circuitBreaker.mu.Unlock()

	cq.circuitBreaker.SuccessCount++
	cq.circuitBreaker.TotalCount++

	// Fermer le circuit si en mode semi-ouvert avec assez de succès
	if cq.circuitBreaker.State == CircuitHalfOpen &&
		cq.circuitBreaker.SuccessCount >= cq.circuitBreaker.SuccessThreshold {
		cq.circuitBreaker.State = CircuitClosed
		cq.circuitBreaker.LastStateChange = time.Now()
		cq.circuitBreaker.FailureCount = 0
		cq.circuitBreaker.SuccessCount = 0
		cq.circuitBreaker.TotalCount = 0
	}
}

// Dequeue récupère un msg de la queue
func (cq *ChannelQueue) Dequeue(ctx context.Context) (*Message, error) {
	select {
	case <-cq.workerCtx.Done():
		return nil, ErrQueueClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-cq.messages:
		// decrease MessageCount si mode non-persistant
		if !cq.queue.Config.IsPersistent {
			cq.queue.MessageCount--
		}
		return msg, nil
	default:
		// Essayer defaçon blocqunte
		select {
		case <-cq.workerCtx.Done():
			return nil, ErrQueueClosed
		case <-ctx.Done():
			return nil, ctx.Err()
		case msg := <-cq.messages:
			if !cq.queue.Config.IsPersistent {
				cq.queue.MessageCount--
			}
			return msg, nil
		default:
			return nil, nil // rien
		}
	}
}

// AddSubscriber ajoute un consumer à une queue
func (cq *ChannelQueue) AddSubscriber(handler MessageHandler) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	cq.subscribers = append(cq.subscribers, handler)
}

// RemoveSubscriber le supprime
func (cq *ChannelQueue) RemoveSubscriber(handler MessageHandler) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	for i, sub := range cq.subscribers {
		// comparaison d'adresses de func (basique mais marche)
		if &sub == &handler {
			cq.subscribers = append(cq.subscribers[:i], cq.subscribers[i+1:]...)
			break
		}
	}
}

// Start démarre les workers pour traiter les messages
func (cq *ChannelQueue) Start(ctx context.Context) {
	// Nombre de workers basé sur lemode de livraison
	workerCount := 1
	if cq.queue.Config.DeliveryMode == BroadcastMode {
		workerCount = 2 // plus pour gérer la diffusion
	}

	for i := 0; i < workerCount; i++ {
		cq.wg.Add(1)
		go func(workerID int) {
			defer cq.wg.Done()
			go cq.processMessages()
		}(i)
	}

	// Si les retries sont activés, démarrer le worker de retry
	if cq.retryQueue != nil && cq.queue.Config.RetryEnabled {
		cq.wg.Add(1)
		go func() {
			defer cq.wg.Done()
			cq.processRetries()
		}()
	}
}

// processMessages traite les messages entrants
func (cq *ChannelQueue) processMessages() {
	for {
		select {
		case <-cq.workerCtx.Done():
			return // Sortir proprement si le contexte est annulé
		case msg, ok := <-cq.messages:
			if !ok {
				// Canal fermé, sortir proprement
				return
			}

			// Acquérir le sémaphore (limiter la concurrence)
			select {
			case cq.workerSem <- struct{}{}:
				// Sémaphore acquis, traiter le message
				go func(m *Message) {
					defer func() {
						// Libérer le sémaphore
						<-cq.workerSem
					}()

					// Décrémenter le compteur si mode non-persistant
					if !cq.queue.Config.IsPersistent {
						cq.queue.MessageCount--
					}

					// Notifier les abonnés selon le mode de livraison
					cq.mu.RLock()
					subscribers := cq.subscribers
					cq.mu.RUnlock()

					switch cq.queue.Config.DeliveryMode {
					case BroadcastMode:
						// Envoyer à tous les abonnés
						for _, handler := range subscribers {
							// Cloner le message pour chaque abonné pour éviter les race conditions
							msgCopy := *m
							if err := handler(&msgCopy); err != nil {
								cq.handleDeliveryError(&msgCopy, handler, err)
							}
						}
					case RoundRobinMode:
						// Améliorer le round-robin avec un index moins prévisible
						if len(subscribers) > 0 {
							idx := int(m.Timestamp.UnixNano()) % len(subscribers)
							handler := subscribers[idx]
							if err := handler(m); err != nil {
								cq.handleDeliveryError(m, handler, err)
							}
						}
					case SingleConsumerMode:
						// Envoyer seulement au premier abonné
						if len(subscribers) > 0 {
							handler := subscribers[0]
							if err := handler(m); err != nil {
								cq.handleDeliveryError(m, handler, err)
							}
						}
					}
				}(msg)
			case <-cq.workerCtx.Done():
				return // Sortir si le contexte est annulé pendant l'attente du sémaphore
			case <-time.After(1 * time.Second):
				// Si le sémaphore est bloqué trop longtemps, loguer et réessayer
				log.Printf("Worker semaphore acquisition timed out for queue %s", cq.queue.Name)
				continue
			}
		}
	}
}

// handleDeliveryError gère les erreurs lors de la distribution des messages
func (cq *ChannelQueue) handleDeliveryError(msg *Message, handler MessageHandler, err error) {
	log.Printf("Error handling message %s: %v", msg.ID, err)

	// Si le circuit breaker est activé, enregistrer l'échec
	if cq.circuitBreaker != nil {
		cq.circuitBreaker.mu.Lock()
		cq.circuitBreaker.FailureCount++
		cq.circuitBreaker.TotalCount++

		// Vérifier si le circuit doit être ouvert
		if cq.circuitBreaker.State == CircuitClosed &&
			cq.circuitBreaker.TotalCount >= cq.circuitBreaker.MinimumRequests {
			errorRate := float64(cq.circuitBreaker.FailureCount) / float64(cq.circuitBreaker.TotalCount)
			if errorRate >= cq.circuitBreaker.ErrorThreshold {
				cq.circuitBreaker.State = CircuitOpen
				cq.circuitBreaker.LastStateChange = time.Now()
				cq.circuitBreaker.NextAttempt = time.Now().Add(cq.circuitBreaker.OpenTimeout)
			}
		} else if cq.circuitBreaker.State == CircuitHalfOpen {
			// En mode semi-ouvert, toute erreur ouvre à nouveau le circuit
			cq.circuitBreaker.State = CircuitOpen
			cq.circuitBreaker.LastStateChange = time.Now()
			cq.circuitBreaker.NextAttempt = time.Now().Add(cq.circuitBreaker.OpenTimeout)
		}
		cq.circuitBreaker.mu.Unlock()
	}

	// Si les retries sont activés, ajouter le message à la file de retry
	if cq.retryQueue != nil && cq.queue.Config.RetryConfig != nil {
		// Récupérer les infos de retry existantes ou créer un nouveau
		retryInfo, ok := msg.Metadata["retry_info"].(*MessageWithRetry)
		if !ok {
			retryInfo = &MessageWithRetry{
				Message:    msg,
				RetryCount: 0,
				Handler:    handler,
			}
		}

		retryInfo.RetryCount++

		// Vérifier si le nombre max de retries est atteint
		if cq.queue.Config.RetryConfig.MaxRetries > 0 &&
			retryInfo.RetryCount > cq.queue.Config.RetryConfig.MaxRetries {
			// Log max retries reached
			return
		}

		// Calculer le délai de retry avec backoff exponentiel
		delay := cq.calculateRetryDelay(retryInfo.RetryCount)
		retryInfo.NextRetryAt = time.Now().Add(delay)

		// Mettre à jour les métadonnées
		if msg.Metadata == nil {
			msg.Metadata = make(map[string]interface{})
		}
		msg.Metadata["retry_info"] = retryInfo

		// Ajouter à la file de retry
		select {
		case cq.retryQueue <- retryInfo:
			// ok
		default:
			// File pleine, log
		}
	}
}

// calculateRetryDelay calcule le délai pour une tentative
func (cq *ChannelQueue) calculateRetryDelay(retryCount int) time.Duration {
	config := cq.queue.Config.RetryConfig
	if config == nil {
		return 5 * time.Second // Valeur par défaut
	}

	initialDelay := config.InitialDelay
	if initialDelay <= 0 {
		initialDelay = 1 * time.Second
	}

	factor := config.Factor
	if factor <= 0 {
		factor = 2.0 // Backoff exponentiel standard
	}

	// Calcul du délai avec backoff exponentiel
	delay := initialDelay * time.Duration(math.Pow(factor, float64(retryCount-1)))

	// Limiter au délai maximum si défini
	if config.MaxDelay > 0 && delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	return delay
}

// processRetries traite les messages en attente de retry
func (cq *ChannelQueue) processRetries() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	pendingRetries := make([]*MessageWithRetry, 0)

	for {
		select {
		case <-cq.workerCtx.Done():
			return
		case retry := <-cq.retryQueue:
			pendingRetries = append(pendingRetries, retry)
		case <-ticker.C:
			now := time.Now()
			remaining := make([]*MessageWithRetry, 0, len(pendingRetries))

			for _, retry := range pendingRetries {
				if now.After(retry.NextRetryAt) {
					// Réessayer le message
					go func(r *MessageWithRetry) {
						if err := r.Handler(r.Message); err != nil {
							// Échec, remettre en retry si possible
							cq.handleDeliveryError(r.Message, r.Handler, err)
						}
					}(retry)
				} else {
					// Pas encore temps de retry
					remaining = append(remaining, retry)
				}
			}

			pendingRetries = remaining
		}
	}
}

// Stop arrête tous les workers et ferme la queue
func (cq *ChannelQueue) Stop() {
	// Annuler le contexte pour signaler l'arrêt à toutes les goroutines
	cq.workerCancel()

	// Utiliser un canal de notification plutôt qu'un timeout fixe
	done := make(chan struct{})
	go func() {
		cq.wg.Wait()
		close(done)
	}()

	// Attendre avec timeout
	select {
	case <-done:
		// Goroutines terminées correctement
		log.Printf("Queue %s stopped cleanly", cq.queue.Name)
	case <-time.After(5 * time.Second):
		// Timeout atteint
		log.Printf("Queue %s stop timed out", cq.queue.Name)
	}

	// Ne pas fermer les canaux car cela peut causer des panics
	// cq.workerCancel() signalera aux goroutines de se terminer
}
