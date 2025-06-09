package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ajkula/GoRTMS/config"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
	"github.com/gorilla/mux"
)

// REST API handler
type Handler struct {
	logger               outbound.Logger
	config               *config.Config
	messageService       inbound.MessageService
	domainService        inbound.DomainService
	queueService         inbound.QueueService
	routingService       inbound.RoutingService
	statsService         inbound.StatsService
	resourceMonitor      inbound.ResourceMonitorService
	consumerGroupService inbound.ConsumerGroupService
	consumerGroupRepo    outbound.ConsumerGroupRepository
}

func NewHandler(
	logger outbound.Logger,
	config *config.Config,
	messageService inbound.MessageService,
	domainService inbound.DomainService,
	queueService inbound.QueueService,
	routingService inbound.RoutingService,
	statsService inbound.StatsService,
	resourceMonitor inbound.ResourceMonitorService,
	consumerGroupService inbound.ConsumerGroupService,
	consumerGroupRepo outbound.ConsumerGroupRepository,
) *Handler {
	return &Handler{
		logger:               logger,
		config:               config,
		messageService:       messageService,
		domainService:        domainService,
		queueService:         queueService,
		routingService:       routingService,
		statsService:         statsService,
		resourceMonitor:      resourceMonitor,
		consumerGroupService: consumerGroupService,
		consumerGroupRepo:    consumerGroupRepo,
	}
}

// SetupRoutes REST API config
func (h *Handler) SetupRoutes(router *mux.Router) {
	// Domains routes
	router.HandleFunc("/api/domains", h.listDomains).Methods("GET")
	router.HandleFunc("/api/domains", h.createDomain).Methods("POST")
	router.HandleFunc("/api/domains/{domain}", h.getDomain).Methods("GET")
	router.HandleFunc("/api/domains/{domain}", h.deleteDomain).Methods("DELETE")

	// Queues routes
	router.HandleFunc("/api/domains/{domain}/queues", h.listQueues).Methods("GET")
	router.HandleFunc("/api/domains/{domain}/queues", h.createQueue).Methods("POST")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}", h.getQueue).Methods("GET")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}", h.deleteQueue).Methods("DELETE")

	// Messages routes
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/messages", h.publishMessage).Methods("POST")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/messages", h.consumeMessages).Methods("GET")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/subscribe", h.subscribeToQueue).Methods("POST")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/unsubscribe", h.unsubscribeFromQueue).Methods("POST")

	// Routing rules routes
	router.HandleFunc("/api/domains/{domain}/routes", h.listRoutingRules).Methods("GET")
	router.HandleFunc("/api/domains/{domain}/routes", h.addRoutingRule).Methods("POST")
	router.HandleFunc("/api/domains/{domain}/routes/{source}/{destination}", h.removeRoutingRule).Methods("DELETE")

	// Simulation routes
	router.HandleFunc("/api/domains/{domain}/routes/test", h.testRoutingRules).Methods("POST")

	// ConsumerGroup routes
	router.HandleFunc("/api/consumer-groups", h.listAllConsumerGroups).Methods("GET")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/consumer-groups", h.listConsumerGroups).Methods("GET")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/consumer-groups", h.createConsumerGroup).Methods("POST")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/consumer-groups/{group}", h.getConsumerGroup).Methods("GET")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/consumer-groups/{group}", h.deleteConsumerGroup).Methods("DELETE")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/consumer-groups/{group}/ttl", h.updateConsumerGroupTTL).Methods("PUT")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/consumer-groups/{group}/messages", h.getPendingMessages).Methods("GET")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/consumer-groups/{group}/consumers", h.addConsumerToGroup).Methods("POST")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/consumer-groups/{group}/consumers/{consumer}", h.removeConsumerFromGroup).Methods("DELETE")

	// Stats routes
	router.HandleFunc("/api/stats", h.getStats).Methods("GET")

	// system ressources routes
	if h.resourceMonitor != nil {
		h.logger.Info("Setting up resource monitoring routes")
		router.HandleFunc("/api/resources/current", h.getCurrentResourceStats).Methods("GET")
		router.HandleFunc("/api/resources/history", h.getResourceStatsHistory).Methods("GET")
		router.HandleFunc("/api/resources/domains/{domain}", h.getDomainResourceStats).Methods("GET")
	}

	// settings routes
	router.HandleFunc("/api/settings", h.getSettings).Methods("GET")
	router.HandleFunc("/api/settings", h.updateSettings).Methods("PUT")
	router.HandleFunc("/api/settings/reset", h.resetSettings).Methods("POST")

	// health check routes
	router.HandleFunc("/health", h.healthCheck).Methods("GET")

	// UI routes
	router.PathPrefix("/ui/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// req file path
		path := strings.TrimPrefix(r.URL.Path, "/ui/")
		filePath := filepath.Join("./web/dist", path)

		// check if file exists
		_, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			// if file doesn't exist, serve index.html for React routing
			http.ServeFile(w, r, "./web/dist/index.html")
			return
		}

		// Or, serve static file
		http.StripPrefix("/ui/", http.FileServer(http.Dir("./web/dist"))).ServeHTTP(w, r)
	}))
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) listDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := h.domainService.ListDomains(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// simple JSON response structure
	type domainResponse struct {
		Name string `json:"name"`
	}

	response := make([]domainResponse, len(domains))
	for i, domain := range domains {
		response[i] = domainResponse{Name: domain.Name}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"domains": response,
	})
}

func (h *Handler) createDomain(w http.ResponseWriter, r *http.Request) {
	var config model.DomainConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.domainService.CreateDomain(r.Context(), &config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Register event
	h.statsService.RecordDomainCreated(config.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"domain": config.Name,
	})
}

func (h *Handler) getDomain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]

	domain, err := h.domainService.GetDomain(r.Context(), domainName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Simple structure circular reference-less
	type QueueInfo struct {
		Name         string            `json:"name"`
		MessageCount int               `json:"messageCount"`
		Config       model.QueueConfig `json:"config"`
	}

	type RouteInfo struct {
		SourceQueue      string `json:"sourceQueue"`
		DestinationQueue string `json:"destinationQueue"`
		Predicate        any    `json:"predicate"`
	}

	type DomainResponse struct {
		Name   string           `json:"name"`
		Schema model.SchemaInfo `json:"schema,omitempty"`
		Queues []QueueInfo      `json:"queues"`
		Routes []RouteInfo      `json:"routes"`
	}

	// assign response
	response := DomainResponse{
		Name:   domain.Name,
		Queues: make([]QueueInfo, 0, len(domain.Queues)),
		Routes: make([]RouteInfo, 0),
	}

	// Convert schema to serializable type
	if domain.Schema != nil {
		schemaInfo := model.SchemaInfo{
			HasValidation: domain.Schema.Validation != nil,
		}

		// Copie if available
		if domain.Schema.Fields != nil {
			schemaInfo.Fields = make(map[string]string)
			for fieldName, fieldType := range domain.Schema.Fields {
				schemaInfo.Fields[fieldName] = string(fieldType)
			}
		}

		response.Schema = schemaInfo
	}

	// Add queues
	for queueName, queue := range domain.Queues {
		response.Queues = append(response.Queues, QueueInfo{
			Name:         queueName,
			MessageCount: queue.MessageCount,
			Config:       queue.Config,
		})
	}

	// Add better predicate treatment routes
	for srcQueue, dstRoutes := range domain.Routes {
		for dstQueue, rule := range dstRoutes {
			var predicateInfo any = nil

			switch pred := rule.Predicate.(type) {
			case model.JSONPredicate:
				// JSONPredicate
				predicateInfo = pred

			case model.PredicateFunc, func(*model.Message) bool:
				// function - unserializable
				predicateInfo = map[string]string{
					"type": "function",
					"info": "Predicate function (non-serializable)",
				}

			case map[string]any:
				// existing map - JSONPredicate ?
				if pred["type"] != nil && pred["field"] != nil {
					// map JSONPredicate
					predicateInfo = map[string]any{
						"type":  pred["type"],
						"field": pred["field"],
						"value": pred["value"],
					}
				} else {
					// Map - keep as is
					predicateInfo = pred
				}

			default:
				// default - serve type for debug
				predicateInfo = map[string]string{
					"type": fmt.Sprintf("%T", rule.Predicate),
					"info": "Unknown predicate type",
				}
			}

			response.Routes = append(response.Routes, RouteInfo{
				SourceQueue:      srcQueue,
				DestinationQueue: dstQueue,
				Predicate:        predicateInfo,
			})
		}
	}

	// Log response
	// respBytes, err := json.MarshalIndent(response, "", "  ")
	// if err != nil {
	// 	h.logger.Error("Error marshaling response", "ERROR", err)
	// 	http.Error(w, "Internal server error", http.StatusInternalServerError)
	// 	return
	// }

	w.Header().Set("Content-Type", "application/json")
	h.logger.Debug("Domain response", "response", response)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) deleteDomain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]

	if err := h.domainService.DeleteDomain(r.Context(), domainName); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

func (h *Handler) listQueues(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]

	queues, err := h.queueService.ListQueues(r.Context(), domainName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// simple JSON response structure
	type queueResponse struct {
		Name         string             `json:"name"`
		MessageCount int                `json:"messageCount"`
		Config       *model.QueueConfig `json:"config"`
	}

	response := make([]queueResponse, len(queues))
	for i, queue := range queues {
		response[i] = queueResponse{
			Name:         queue.Name,
			MessageCount: queue.MessageCount,
			Config:       &queue.Config,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"queues": response,
	})
}

func (h *Handler) createQueue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]

	// Read raw req body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Error reading request body", "ERROR", err)
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	// Reset body for JSON decoder
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Log req body
	h.logger.Debug("Queue creation request", "body", string(bodyBytes))

	var request struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"` // RawMessage to avoid decoding pblms
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.logger.Error("Error decoding request JSON", "ERROR", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// decode manually
	var configMap map[string]any
	if err := json.Unmarshal(request.Config, &configMap); err != nil {
		h.logger.Error("Error decoding config", "ERROR", err)
		http.Error(w, "Invalid config format", http.StatusBadRequest)
		return
	}

	// Base config
	config := &model.QueueConfig{}

	// Apply req values
	if isPersistent, ok := configMap["isPersistent"].(bool); ok {
		config.IsPersistent = isPersistent
	}

	// JSON makes numbers as float64
	if maxSize, ok := configMap["maxSize"].(float64); ok {
		config.MaxSize = int(maxSize)
	}

	if ttlStr, ok := configMap["ttl"].(string); ok {
		ttl, err := time.ParseDuration(ttlStr)
		if err != nil {
			h.logger.Error("Error parsing TTL duration", "ERROR", err)
			// use default instead
		} else {
			config.TTL = ttl
		}
	}

	// Process delivery mode
	if modeStr, ok := configMap["deliveryMode"].(string); ok {
		switch modeStr {
		case "broadcast":
			config.DeliveryMode = model.BroadcastMode
		case "roundRobin":
			config.DeliveryMode = model.RoundRobinMode
		case "singleConsumer":
			config.DeliveryMode = model.SingleConsumerMode
		default:
			h.logger.Warn("Unknown delivery mode, using default", "mode", modeStr)
			config.DeliveryMode = model.BroadcastMode
		}
	}

	h.logger.Debug("Creating queue", "config", config)

	// Process retry config
	if retryEnabled, ok := configMap["retryEnabled"].(bool); ok && retryEnabled {
		config.RetryEnabled = true
		if retryConfigMap, ok := configMap["retryConfig"].(map[string]interface{}); ok {
			retryConfig := &model.RetryConfig{}

			if v, ok := retryConfigMap["maxRetries"].(float64); ok {
				retryConfig.MaxRetries = int(v)
			}

			if v, ok := retryConfigMap["factor"].(float64); ok {
				retryConfig.Factor = v
			}

			if v, ok := retryConfigMap["initialDelay"].(string); ok {
				if d, err := time.ParseDuration(v); err == nil {
					retryConfig.InitialDelay = d
				}
			}

			if v, ok := retryConfigMap["maxDelay"].(string); ok {
				if d, err := time.ParseDuration(v); err == nil {
					retryConfig.MaxDelay = d
				}
			}

			config.RetryConfig = retryConfig
		}
	}

	// Process circuit breaker config
	if cbEnabled, ok := configMap["circuitBreakerEnabled"].(bool); ok && cbEnabled {
		config.CircuitBreakerEnabled = true
		if cbConfigMap, ok := configMap["circuitBreakerConfig"].(map[string]interface{}); ok {
			cbConfig := &model.CircuitBreakerConfig{}

			if v, ok := cbConfigMap["errorThreshold"].(float64); ok {
				cbConfig.ErrorThreshold = v
			}

			if v, ok := cbConfigMap["minimumRequests"].(float64); ok {
				cbConfig.MinimumRequests = int(v)
			}

			if v, ok := cbConfigMap["successThreshold"].(float64); ok {
				cbConfig.SuccessThreshold = int(v)
			}

			if v, ok := cbConfigMap["openTimeout"].(string); ok {
				if d, err := time.ParseDuration(v); err == nil {
					cbConfig.OpenTimeout = d
				}
			}

			config.CircuitBreakerConfig = cbConfig
		}
	}

	if err := h.queueService.CreateQueue(r.Context(), domainName, request.Name, config); err != nil {
		h.logger.Error("Error from service", "ERROR", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type CreateQueueResponse struct {
		Status string             `json:"status"`
		Queue  string             `json:"queue"`
		Config *model.QueueConfig `json:"config"`
	}

	response := CreateQueueResponse{
		Status: "success",
		Queue:  request.Name,
		Config: config,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) getQueue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]

	queue, err := h.queueService.GetQueue(r.Context(), domainName, queueName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":         queue.Name,
		"messageCount": queue.MessageCount,
		"config":       queue.Config,
	})
}

func (h *Handler) deleteQueue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]

	if err := h.queueService.DeleteQueue(r.Context(), domainName, queueName); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

func (h *Handler) publishMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.logger.Error("Error decoding request body", "ERROR", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.logger.Debug("Message payload", "payload", fmt.Sprintf("%+v", payload))

	// Convert to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("Error marshalling payload", "ERROR", err)
		http.Error(w, "Failed to encode payload", http.StatusInternalServerError)
		return
	}

	_, err = h.queueService.GetQueue(r.Context(), domainName, queueName)
	if err != nil {
		h.logger.Error("Error retrieving queue",
			"queue", queueName,
			"ERROR", err)
		http.Error(w, fmt.Sprintf("Queue not found: %s", err), http.StatusNotFound)
		return
	}

	id := GenerateID()
	ID, exists := payload["id"].(string)
	if exists {
		id = ID
	}

	// Create message
	message := &model.Message{
		ID:        id,
		Payload:   payloadBytes,
		Headers:   extractHeaders(r),
		Timestamp: time.Now(),
	}

	// Publish message
	if err := h.messageService.PublishMessage(domainName, queueName, message); err != nil {
		h.logger.Error("Error publishing message", "ERROR", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "success",
		"messageId": message.ID,
	})
}

func (h *Handler) consumeMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]

	query := r.URL.Query()
	timeoutStr := query.Get("timeout")
	maxCountStr := query.Get("max")
	groupID := query.Get("group")
	startFromID := query.Get("start_from")
	consumerID := query.Get("consumer")

	timeout := 0
	if timeoutStr != "" {
		timeout, _ = strconv.Atoi(timeoutStr)
	}

	maxCount := 1
	if maxCountStr != "" {
		maxCount, _ = strconv.Atoi(maxCountStr)
	}

	h.logger.Debug("Received request",
		"group", groupID,
		"consumer", consumerID,
		"maxCount", maxCount)

	// long polling if timeout is set TODO: check this part
	ctx := r.Context()
	if timeout > 0 {
		var cancel context.CancelFunc
		_, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	var messages []*model.Message

	if groupID == "" {
		groupID = "temp-" + time.Now().Format("20060102-150405.999999999")
	}
	options := &inbound.ConsumeOptions{
		StartFromID: startFromID,
		ConsumerID:  consumerID,
		Timeout:     time.Duration(timeout) * time.Second,
	}

	for range maxCount {
		message, err := h.messageService.ConsumeMessageWithGroup(ctx, domainName, queueName, groupID, options)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if message == nil {
			break // empty
		}

		messages = append(messages, message)
	}

	responseMessages := make([]map[string]any, len(messages))
	for i, msg := range messages {
		var payload map[string]any
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			payload = map[string]any{"data": string(msg.Payload)}
		}

		// Add metadata
		responseMsg := map[string]any{
			"id":        msg.ID,
			"timestamp": msg.Timestamp,
			"headers":   msg.Headers,
		}

		// Fusion with payload
		for k, v := range payload {
			responseMsg[k] = v
		}

		responseMessages[i] = responseMsg
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"messages": responseMessages,
		"count":    len(messages),
	})
}

// TODO: check this
func (h *Handler) subscribeToQueue(w http.ResponseWriter, r *http.Request) {
	// vars := mux.Vars(r)
	// domainName := vars["domain"]
	// queueName := vars["queue"]

	var request struct {
		CallbackURL string `json:"callbackUrl,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		// Ignore err, might be empty body
	}

	subscriptionID := GenerateID()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":         "success",
		"subscriptionId": subscriptionID,
		"message":        "Use WebSocket for real-time messages",
	})
}

func (h *Handler) unsubscribeFromQueue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]

	var request struct {
		SubscriptionID string `json:"subscriptionId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.messageService.UnsubscribeFromQueue(domainName, queueName, request.SubscriptionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

func (h *Handler) listRoutingRules(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]

	rules, err := h.routingService.ListRoutingRules(r.Context(), domainName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"rules": rules,
	})
}

func (h *Handler) addRoutingRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]

	var rule model.RoutingRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.routingService.AddRoutingRule(r.Context(), domainName, &rule); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

func (h *Handler) removeRoutingRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	sourceQueue := vars["source"]
	destQueue := vars["destination"]

	if err := h.routingService.RemoveRoutingRule(r.Context(), domainName, sourceQueue, destQueue); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters with defaults
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "1h" // Default: last hour
	}

	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "auto" // Auto-adapt based on period
	}

	// Get aggregated stats
	stats, err := h.statsService.GetStatsWithAggregation(ctx, period, granularity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// extracts meaningful headers from req
func extractHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)

	relevantHeaders := []string{
		"Content-Type",
		"X-Request-ID",
		"User-Agent",
	}

	for _, header := range relevantHeaders {
		if value := r.Header.Get(header); value != "" {
			headers[header] = value
		}
	}

	return headers
}

// returns ressources usage stats
func (h *Handler) getCurrentResourceStats(w http.ResponseWriter, r *http.Request) {
	if h.resourceMonitor == nil {
		http.Error(w, "Resource monitoring not available", http.StatusServiceUnavailable)
		return
	}

	stats, err := h.resourceMonitor.GetCurrentStats(r.Context())
	if err != nil {
		h.logger.Error("Error getting current resource stats", "ERROR", err)
		http.Error(w, "Failed to get resource statistics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) getResourceStatsHistory(w http.ResponseWriter, r *http.Request) {
	if h.resourceMonitor == nil {
		http.Error(w, "Resource monitoring not available", http.StatusServiceUnavailable)
		return
	}

	// Optional param to limit points number
	limitStr := r.URL.Query().Get("limit")
	limit := 0

	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
			return
		}
	}

	stats, err := h.resourceMonitor.GetStatsHistory(r.Context(), limit)
	if err != nil {
		h.logger.Error("Error getting resource stats history", "ERROR", err)
		http.Error(w, "Failed to get resource statistics history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) getDomainResourceStats(w http.ResponseWriter, r *http.Request) {
	if h.resourceMonitor == nil {
		http.Error(w, "Resource monitoring not available", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	domainName := vars["domain"]

	stats, err := h.resourceMonitor.GetCurrentStats(r.Context())
	if err != nil {
		h.logger.Error("Error getting current resource stats", "ERROR", err)
		http.Error(w, "Failed to get resource statistics", http.StatusInternalServerError)
		return
	}

	domainStats, exists := stats.DomainStats[domainName]
	if !exists {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domainStats)
}

func GenerateID() string {
	return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}
