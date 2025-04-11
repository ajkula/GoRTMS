package model

import (
	"context"
	"errors"
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
}

// NewChannelQueue crée une nouvelle queue basée sur des channels
func NewChannelQueue(queue *Queue, ctx context.Context, bufferSize int) *ChannelQueue {
	if bufferSize <= 0 {
		bufferSize = 100 // default
	}

	workerCtx, cancel := context.WithCancel(ctx)

	return &ChannelQueue{
		queue:        queue,
		messages:     make(chan *Message, bufferSize),
		subscribers:  make([]MessageHandler, 0),
		workerCtx:    workerCtx,
		workerCancel: cancel,
		bufferSize:   bufferSize,
	}
}

// GetQueue return la queue sous-jacente
func (cq *ChannelQueue) GetQueue() *Queue {
	return cq.queue
}

// Enqueue ajoute un message à la queue
func (cq *ChannelQueue) Enqueue(ctx context.Context, message *Message) error {
	select {
	case <-cq.workerCtx.Done():
		return ErrQueueClosed
	case <-ctx.Done():
		return ctx.Err()
	case cq.messages <- message:
		// update MessageCount
		cq.queue.MessageCount++
		return nil
	default:
		// si taile max et pleine
		if cq.queue.Config.MaxSize > 0 && cq.queue.MessageCount >= cq.queue.Config.MaxSize {
			return ErrQueueFull
		}

		// sino, essaye d'ajouter de manière blaquante avec timeout
		timer := time.NewTimer((5000 * time.Millisecond))
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
			return nil
		}
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
		go cq.processMessages()
	}
}

// processMessages traite les messages entrants
func (cq *ChannelQueue) processMessages() {
	for {
		select {
		case <-cq.workerCtx.Done():
			return
		case msg := <-cq.messages:
			// Décrémenter le compteur si mode non-persistant
			if !cq.queue.Config.IsPersistent {
				cq.queue.MessageCount--
			}

			// Notif. les abonnés selon le mode de livraison
			cq.mu.RLock()
			subscribers := cq.subscribers
			cq.mu.RUnlock()

			switch cq.queue.Config.DeliveryMode {
			case BroadcastMode:
				// Implémenter round-robin entre les abonnés
				if len(subscribers) > 0 {
					// implémentation simple basé sur l'index du message
					idx := int(msg.Timestamp.UnixNano()) % len(subscribers)
					subscribers[idx](msg)
				}

			case SingleConsumerMode:
				// Envoyer seulement au premier abonné
				if len(subscribers) > 0 {
					subscribers[0](msg)
				}
			}
		}
	}
}

// Stop arrête tous les workers et ferme la queue
func (cq *ChannelQueue) Stop() {
	cq.workerCancel()
	// Ne pas fermer le chan ici sinon panic
	// le context Cancel arrêtera les goroutines
}
