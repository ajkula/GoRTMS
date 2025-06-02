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
	queue           *Queue
	messages        chan *Message
	subscribers     []MessageHandler
	workerCtx       context.Context
	workerCancel    context.CancelFunc
	bufferSize      int
	messageProvider MessageProvider
	domainName      string
	logger          Logger

	wg        sync.WaitGroup // workers
	workerSem chan struct{}  // simultaneous goroutines controling semaphore

	// errors handling
	retryQueue     chan *MessageWithRetry
	circuitBreaker *CircuitBreaker

	consumerGroups map[string]*ConsumerGroupState
	mu             sync.RWMutex
	commandWorker  bool

	pendingFetches map[string]bool // groupID -> isCurrentlyFetching
	fetchMu        sync.Mutex
}

type ConsumerGroupState struct {
	GroupID  string
	Messages chan *Message // messages chan
	Commands chan int      // commands chan
	Position int64
	Active   bool
}

func NewChannelQueue(
	ctx context.Context,
	logger Logger,
	queue *Queue,
	bufferSize int,
	provider MessageProvider,
) *ChannelQueue {
	if bufferSize <= 0 {
		bufferSize = queue.Config.MaxSize
		if queue.Config.MaxSize <= 0 {
			bufferSize = 1000
		}
	}

	workerCtx, cancel := context.WithCancel(ctx)

	workerCount := queue.Config.WorkerCount
	if workerCount <= 0 {
		// Use a default number based on the delivery mode
		if queue.Config.DeliveryMode == BroadcastMode {
			workerCount = 2
		} else {
			workerCount = 1
		}
	}

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

		if cb.ErrorThreshold <= 0 {
			cb.ErrorThreshold = 0.5
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

	var retryQueue chan *MessageWithRetry
	if queue.Config.RetryEnabled {
		retryQueue = make(chan *MessageWithRetry, bufferSize)
	}

	return &ChannelQueue{
		queue:           queue,
		messages:        make(chan *Message, bufferSize),
		subscribers:     make([]MessageHandler, 0),
		workerCtx:       workerCtx,
		workerCancel:    cancel,
		bufferSize:      bufferSize,
		wg:              sync.WaitGroup{},
		workerSem:       make(chan struct{}, workerCount),
		retryQueue:      retryQueue,
		circuitBreaker:  cb,
		consumerGroups:  make(map[string]*ConsumerGroupState),
		messageProvider: provider,
		domainName:      queue.DomainName,
		pendingFetches:  make(map[string]bool),
		logger:          logger,
	}
}

func (cq *ChannelQueue) GetQueue() *Queue {
	return cq.queue
}

func (cq *ChannelQueue) Enqueue(ctx context.Context, message *Message) error {
	// Check circuit breaker state
	if cq.circuitBreaker != nil && cq.circuitBreaker.State == CircuitOpen {
		return errors.New("circuit breaker open, message rejected")
	}

	select {
	case <-cq.workerCtx.Done():
		return ErrQueueClosed
	case <-ctx.Done():
		return ctx.Err()
	case cq.messages <- message:
		// Store success
		if cq.circuitBreaker != nil {
			cq.recordSuccessInCircuitBreaker()
		}

		cq.queue.MessageCount++

		return nil
	default:
		// fails aren't critical
		return nil
	}
}

// helper method
func (cq *ChannelQueue) recordSuccessInCircuitBreaker() {
	cq.circuitBreaker.mu.Lock()
	defer cq.circuitBreaker.mu.Unlock()

	cq.circuitBreaker.SuccessCount++
	cq.circuitBreaker.TotalCount++

	// Close the circuit if in half-open mode with enough successes
	if cq.circuitBreaker.State == CircuitHalfOpen &&
		cq.circuitBreaker.SuccessCount >= cq.circuitBreaker.SuccessThreshold {
		cq.circuitBreaker.State = CircuitClosed
		cq.circuitBreaker.LastStateChange = time.Now()
		cq.circuitBreaker.FailureCount = 0
		cq.circuitBreaker.SuccessCount = 0
		cq.circuitBreaker.TotalCount = 0
	}
}

func (cq *ChannelQueue) Dequeue(ctx context.Context) (*Message, error) {
	select {
	case <-cq.workerCtx.Done():
		return nil, ErrQueueClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-cq.messages:
		if cq.queue.MessageCount > 0 {
			cq.queue.MessageCount--
		}
		return msg, nil
	default:
		// empty chan = need to send command
		return nil, nil
	}
}

func (cq *ChannelQueue) AddConsumerGroup(groupID string, lastIndex int64) error {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	if _, exists := cq.consumerGroups[groupID]; exists {
		return nil // exists
	}

	// Create the group's state with its own channels
	bufSize := cq.bufferSize
	if bufSize <= 0 {
		bufSize = 100
	}

	group := &ConsumerGroupState{
		GroupID:  groupID,
		Messages: make(chan *Message, bufSize),
		Commands: make(chan int, 10), // commands buffer
		Position: lastIndex,
		Active:   true,
	}

	cq.consumerGroups[groupID] = group

	// Start commands worker
	if !cq.commandWorker {
		cq.commandWorker = true
		cq.wg.Add(1)
		go cq.processCommands()
	}

	return nil
}

func (cq *ChannelQueue) processCommands() {
	defer cq.wg.Done()

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cq.workerCtx.Done():
			return

		case <-ticker.C:
			// Check all group commands
			cq.mu.RLock()
			for groupID, group := range cq.consumerGroups {
				if !group.Active {
					continue
				}

				select {
				case count, ok := <-group.Commands:
					if !ok {
						continue
					}
					// Process the command outside the lock
					go cq.fillGroupChannel(groupID, count)
				default:
					// noop
				}
			}
			cq.mu.RUnlock()
		}
	}
}

func (q *ChannelQueue) UpdateConsumerGroupPosition(groupID string, position int64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if group, exists := q.consumerGroups[groupID]; exists {
		if position > group.Position {
			group.Position = position
		}
	}
}

func (cq *ChannelQueue) fillGroupChannel(groupID string, count int) {
	// Check if a fetch is already in progress to avoid concurrent calls
	cq.fetchMu.Lock()
	if cq.pendingFetches[groupID] {
		cq.fetchMu.Unlock()
		return // noop
	}
	cq.pendingFetches[groupID] = true
	cq.fetchMu.Unlock()

	// Ensure cleanup on exit
	defer func() {
		cq.fetchMu.Lock()
		delete(cq.pendingFetches, groupID)
		cq.fetchMu.Unlock()
	}()

	// Get the group state thread-safe
	cq.mu.RLock()
	group, exists := cq.consumerGroups[groupID]
	if !exists || !group.Active {
		cq.mu.RUnlock()
		return
	}
	position := group.Position
	cq.mu.RUnlock()

	// Skip fetch if message channel isn't empty
	if len(group.Messages) > 0 {
		return
	}

	messages, err := cq.messageProvider.GetMessagesAfterIndex(
		cq.workerCtx, cq.domainName, cq.queue.Name, position, count)
	if err != nil {
		log.Printf("Error getting messages: %v", err)
		return
	}

	if len(messages) == 0 {
		return
	}

	for _, msg := range messages {
		select {
		case <-cq.workerCtx.Done():
			return
		case group.Messages <- msg:
		case <-time.After(100 * time.Millisecond):
			// is channel blocked diagnostic
			log.Printf("[WARN] Canal de messages plein pour group=%s", groupID)
			return // full, noop
		}
	}
}

func (cq *ChannelQueue) RemoveConsumerGroup(groupID string) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	if group, exists := cq.consumerGroups[groupID]; exists {
		group.Active = false

		close(group.Messages)
		close(group.Commands)

		delete(cq.consumerGroups, groupID)
	}
}

func (cq *ChannelQueue) RequestMessages(groupID string, count int) error {
	cq.mu.RLock()
	group, exists := cq.consumerGroups[groupID]
	cq.mu.RUnlock()

	if !exists || !group.Active {
		return errors.New("consumer group not active")
	}

	// send command
	select {
	case group.Commands <- count:
		return nil
	case <-time.After(100 * time.Millisecond):
		return errors.New("command channel full")
	}
}

func (cq *ChannelQueue) ConsumeMessage(groupID string, timeout time.Duration) (*Message, error) {
	cq.mu.RLock()
	group, exists := cq.consumerGroups[groupID]
	cq.mu.RUnlock()

	if !exists || !group.Active {
		return nil, errors.New("consumer group not active")
	}

	select {
	case <-cq.workerCtx.Done():
		return nil, ErrQueueClosed
	case msg, ok := <-group.Messages:
		if !ok {
			return nil, ErrQueueClosed
		}
		return msg, nil
	case <-time.After(timeout):
		return nil, nil // Timeout
	}
}

func (cq *ChannelQueue) AddSubscriber(handler MessageHandler) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	cq.subscribers = append(cq.subscribers, handler)
}

func (cq *ChannelQueue) RemoveSubscriber(handler MessageHandler) {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	for i, sub := range cq.subscribers {
		// Compare func addresses (basic but works)
		if &sub == &handler {
			cq.subscribers = append(cq.subscribers[:i], cq.subscribers[i+1:]...)
			break
		}
	}
}

func (cq *ChannelQueue) Start(ctx context.Context) {
	workerCount := 1
	if cq.queue.Config.DeliveryMode == BroadcastMode {
		workerCount = 2
	}

	for i := 0; i < workerCount; i++ {
		cq.wg.Add(1)
		go func(workerID int) {
			defer cq.wg.Done()
			go cq.processMessages()
		}(i)
	}

	// Start retry worker if retries are enabled
	if cq.retryQueue != nil && cq.queue.Config.RetryEnabled {
		cq.wg.Add(1)
		go func() {
			defer cq.wg.Done()
			cq.processRetries()
		}()
	}
}

func (cq *ChannelQueue) processMessages() {
	for {
		select {
		case <-cq.workerCtx.Done():
			return // Exit cleanly if cancelled context
		case msg, ok := <-cq.messages:
			if !ok {
				// Closed, noop
				return
			}

			// Acquire semaphore (limit concurrency)
			select {
			case cq.workerSem <- struct{}{}:
				go func(m *Message) {
					defer func() {
						// release semaphore
						<-cq.workerSem
					}()

					// Notify subscribers based on delivery mode
					cq.mu.RLock()
					subscribers := cq.subscribers
					cq.mu.RUnlock()

					switch cq.queue.Config.DeliveryMode {
					case BroadcastMode:
						for _, handler := range subscribers {
							// Clone the message for each subscriber to avoid race conditions
							msgCopy := *m
							if err := handler(&msgCopy); err != nil {
								cq.handleDeliveryError(&msgCopy, handler, err)
							}
						}
					case RoundRobinMode:
						// Improve round-robin with a less predictable index
						if len(subscribers) > 0 {
							idx := int(m.Timestamp.UnixNano()) % len(subscribers)
							handler := subscribers[idx]
							if err := handler(m); err != nil {
								cq.handleDeliveryError(m, handler, err)
							}
						}
					case SingleConsumerMode:
						// Send to the first available subscriber
						if len(subscribers) > 0 {
							handler := subscribers[0]
							if err := handler(m); err != nil {
								cq.handleDeliveryError(m, handler, err)
							}
						}
					}
				}(msg)
			case <-cq.workerCtx.Done():
				return // Exit if context was canceled while waiting for the semaphore
			case <-time.After(1 * time.Second):
				// If semaphore is blocked too long, log and retry
				log.Printf("Worker semaphore acquisition timed out for queue %s", cq.queue.Name)
				continue
			}
		}
	}
}

func (cq *ChannelQueue) handleDeliveryError(msg *Message, handler MessageHandler, err error) {
	log.Printf("Error handling message %s: %v", msg.ID, err)

	// If circuit breaker is enabled, record the failure
	if cq.circuitBreaker != nil {
		cq.circuitBreaker.mu.Lock()
		cq.circuitBreaker.FailureCount++
		cq.circuitBreaker.TotalCount++

		// Check if the circuit should be opened
		if cq.circuitBreaker.State == CircuitClosed &&
			cq.circuitBreaker.TotalCount >= cq.circuitBreaker.MinimumRequests {
			errorRate := float64(cq.circuitBreaker.FailureCount) / float64(cq.circuitBreaker.TotalCount)
			if errorRate >= cq.circuitBreaker.ErrorThreshold {
				cq.circuitBreaker.State = CircuitOpen
				cq.circuitBreaker.LastStateChange = time.Now()
				cq.circuitBreaker.NextAttempt = time.Now().Add(cq.circuitBreaker.OpenTimeout)
			}
		} else if cq.circuitBreaker.State == CircuitHalfOpen {
			// In half-open mode, any error reopens the circuit
			cq.circuitBreaker.State = CircuitOpen
			cq.circuitBreaker.LastStateChange = time.Now()
			cq.circuitBreaker.NextAttempt = time.Now().Add(cq.circuitBreaker.OpenTimeout)
		}
		cq.circuitBreaker.mu.Unlock()
	}

	// If retries are enabled, add the message to the retry queue
	if cq.retryQueue != nil && cq.queue.Config.RetryConfig != nil {
		// Get existing retry info or create a new one
		retryInfo, ok := msg.Metadata["retry_info"].(*MessageWithRetry)
		if !ok {
			retryInfo = &MessageWithRetry{
				Message:    msg,
				RetryCount: 0,
				Handler:    handler,
			}
		}

		retryInfo.RetryCount++

		// Check if the maximum number of retries has been reached
		if cq.queue.Config.RetryConfig.MaxRetries > 0 &&
			retryInfo.RetryCount > cq.queue.Config.RetryConfig.MaxRetries {
			// Log max retries reached
			return
		}

		// Compute retry delay using exponential backoff
		delay := cq.calculateRetryDelay(retryInfo.RetryCount)
		retryInfo.NextRetryAt = time.Now().Add(delay)

		// update metadata
		if msg.Metadata == nil {
			msg.Metadata = make(map[string]interface{})
		}
		msg.Metadata["retry_info"] = retryInfo

		// Add to retry queue
		select {
		case cq.retryQueue <- retryInfo:
			// ok
		default:
			// Full, should log
		}
	}
}

func (cq *ChannelQueue) calculateRetryDelay(retryCount int) time.Duration {
	config := cq.queue.Config.RetryConfig
	if config == nil {
		return 5 * time.Second // default val
	}

	initialDelay := config.InitialDelay
	if initialDelay <= 0 {
		initialDelay = 1 * time.Second
	}

	factor := config.Factor
	if factor <= 0 {
		factor = 2.0 // Standard exponential backoff
	}

	// Compute delay using exponential backoff
	delay := initialDelay * time.Duration(math.Pow(factor, float64(retryCount-1)))

	// Cap to max delay if defined
	if config.MaxDelay > 0 && delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	return delay
}

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
					// Retry
					go func(r *MessageWithRetry) {
						if err := r.Handler(r.Message); err != nil {
							// Failure, requeue for retry if possible
							cq.handleDeliveryError(r.Message, r.Handler, err)
						}
					}(retry)
				} else {
					// Not time to retry yet
					remaining = append(remaining, retry)
				}
			}

			pendingRetries = remaining
		}
	}
}

func (cq *ChannelQueue) GetBufferStats() (currentSize int, capacity int) {
	return len(cq.messages), cq.bufferSize
}

func (cq *ChannelQueue) Stop() {
	// Cancel context to signal all goroutines to stop
	cq.workerCancel()

	// Use a notification channel instead of a fixed timeout
	done := make(chan struct{})
	go func() {
		cq.wg.Wait()
		close(done)
	}()

	// wait timeout
	select {
	case <-done:
		// Goroutines properly terminated
		log.Printf("Queue %s stopped cleanly", cq.queue.Name)
	case <-time.After(5 * time.Second):
		// Timeout reached
		log.Printf("Queue %s stop timed out", cq.queue.Name)
	}

	// Do not close channels as it may cause panics
	// cq.workerCancel() will signal goroutines to exit
}
