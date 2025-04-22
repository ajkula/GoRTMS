package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/gorilla/mux"
)

// Handler gère les requêtes HTTP pour l'API REST
type Handler struct {
	messageService  inbound.MessageService
	domainService   inbound.DomainService
	queueService    inbound.QueueService
	routingService  inbound.RoutingService
	statsService    inbound.StatsService
	resourceMonitor inbound.ResourceMonitorService
}

// NewHandler crée un nouveau gestionnaire REST
func NewHandler(
	messageService inbound.MessageService,
	domainService inbound.DomainService,
	queueService inbound.QueueService,
	routingService inbound.RoutingService,
	statsService inbound.StatsService,
	resourceMonitor inbound.ResourceMonitorService,
) *Handler {
	return &Handler{
		messageService:  messageService,
		domainService:   domainService,
		queueService:    queueService,
		routingService:  routingService,
		statsService:    statsService,
		resourceMonitor: resourceMonitor,
	}
}

// SetupRoutes configure les routes de l'API REST
func (h *Handler) SetupRoutes(router *mux.Router) {
	// Routes pour les domaines
	router.HandleFunc("/api/domains", h.listDomains).Methods("GET")
	router.HandleFunc("/api/domains", h.createDomain).Methods("POST")
	router.HandleFunc("/api/domains/{domain}", h.getDomain).Methods("GET")
	router.HandleFunc("/api/domains/{domain}", h.deleteDomain).Methods("DELETE")

	// Routes pour les files d'attente
	router.HandleFunc("/api/domains/{domain}/queues", h.listQueues).Methods("GET")
	router.HandleFunc("/api/domains/{domain}/queues", h.createQueue).Methods("POST")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}", h.getQueue).Methods("GET")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}", h.deleteQueue).Methods("DELETE")

	// Routes pour les messages
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/messages", h.publishMessage).Methods("POST")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/messages", h.consumeMessages).Methods("GET")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/subscribe", h.subscribeToQueue).Methods("POST")
	router.HandleFunc("/api/domains/{domain}/queues/{queue}/unsubscribe", h.unsubscribeFromQueue).Methods("POST")

	// Routes pour les règles de routage
	router.HandleFunc("/api/domains/{domain}/routes", h.listRoutingRules).Methods("GET")
	router.HandleFunc("/api/domains/{domain}/routes", h.addRoutingRule).Methods("POST")
	router.HandleFunc("/api/domains/{domain}/routes/{source}/{destination}", h.removeRoutingRule).Methods("DELETE")

	// Simulation de routing
	router.HandleFunc("/api/domains/{domain}/routes/test", h.testRoutingRules).Methods("POST")

	// Route pour les stats
	router.HandleFunc("/api/stats", h.getStats).Methods("GET")

	// Routes pour les ressources système (nouvelles)
	if h.resourceMonitor != nil {
		log.Println("Setting up resource monitoring routes")
		router.HandleFunc("/api/resources/current", h.getCurrentResourceStats).Methods("GET")
		router.HandleFunc("/api/resources/history", h.getResourceStatsHistory).Methods("GET")
		router.HandleFunc("/api/resources/domains/{domain}", h.getDomainResourceStats).Methods("GET")
	}

	// Route pour la santé
	router.HandleFunc("/health", h.healthCheck).Methods("GET")

	// Route pour l'UI
	router.PathPrefix("/ui/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Chemin du fichier demandé
		path := strings.TrimPrefix(r.URL.Path, "/ui/")
		filePath := filepath.Join("./web/dist", path)

		// Vérifier si le fichier existe
		_, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			// Si le fichier n'existe pas, servir index.html pour le routage React
			http.ServeFile(w, r, "./web/dist/index.html")
			return
		}

		// Sinon, servir le fichier statique
		http.StripPrefix("/ui/", http.FileServer(http.Dir("./web/dist"))).ServeHTTP(w, r)
	}))
}

// healthCheck vérifie l'état du service
func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// listDomains liste tous les domaines
func (h *Handler) listDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := h.domainService.ListDomains(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convertir en structure simple pour la réponse JSON
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

// createDomain crée un nouveau domaine
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

	// Enregistrer l'événement
	h.statsService.RecordDomainCreated(config.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"domain": config.Name,
	})
}

// getDomain récupère les détails d'un domaine
func (h *Handler) getDomain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]

	domain, err := h.domainService.GetDomain(r.Context(), domainName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Créer une structure simplifiée sans références circulaires
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

	// Remplir la réponse
	response := DomainResponse{
		Name:   domain.Name,
		Queues: make([]QueueInfo, 0, len(domain.Queues)),
		Routes: make([]RouteInfo, 0),
	}

	// Convertir le schema en version sérialisable
	if domain.Schema != nil {
		schemaInfo := model.SchemaInfo{
			HasValidation: domain.Schema.Validation != nil,
		}

		// Copier les champs si disponibles
		if domain.Schema.Fields != nil {
			schemaInfo.Fields = make(map[string]string)
			for fieldName, fieldType := range domain.Schema.Fields {
				schemaInfo.Fields[fieldName] = string(fieldType)
			}
		}

		response.Schema = schemaInfo
	}

	log.Printf("Domain Queues type: %T", domain.Queues)
	log.Printf("Queue count: %d", len(domain.Queues))
	for qName, q := range domain.Queues {
		log.Printf("Queue %s: Implementation config: %T, MessageCount: %d",
			qName, q.Config, q.MessageCount)
	}

	// Ajouter les queues
	for queueName, queue := range domain.Queues {
		response.Queues = append(response.Queues, QueueInfo{
			Name:         queueName,
			MessageCount: queue.MessageCount,
			Config:       queue.Config,
		})
	}

	// Ajouter les routes avec un meilleur traitement des prédicats
	for srcQueue, dstRoutes := range domain.Routes {
		for dstQueue, rule := range dstRoutes {
			var predicateInfo any = nil

			switch pred := rule.Predicate.(type) {
			case model.JSONPredicate:
				// Cas du JSONPredicate explicite
				predicateInfo = pred

			case model.PredicateFunc, func(*model.Message) bool:
				// Cas d'une fonction - non sérialisable
				predicateInfo = map[string]string{
					"type": "function",
					"info": "Predicate function (non-serializable)",
				}

			case map[string]any:
				// Cas d'un map existant - probablement déjà un prédicat structuré
				// Vérifier s'il a la structure d'un JSONPredicate
				if pred["type"] != nil && pred["field"] != nil {
					// C'est probablement un JSONPredicate sous forme de map
					predicateInfo = map[string]any{
						"type":  pred["type"],
						"field": pred["field"],
						"value": pred["value"],
					}
				} else {
					// Map générique - conserver tel quel
					predicateInfo = pred
				}

			default:
				// Cas par défaut - fournir le type pour le débogage
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

	// Log la réponse pour débogage
	respBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	log.Printf("Domain response: %s", string(respBytes))

	w.Header().Set("Content-Type", "application/json")
	log.Printf("lareponse: %v", response)
	json.NewEncoder(w).Encode(response)
}

// deleteDomain supprime un domaine
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

// listQueues liste toutes les files d'attente d'un domaine
func (h *Handler) listQueues(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]

	queues, err := h.queueService.ListQueues(r.Context(), domainName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convertir en structure simple pour la réponse JSON
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

// createQueue crée une nouvelle file d'attente
func (h *Handler) createQueue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]

	// Lire le corps de la requête brut
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	// Réinitialiser le corps pour le décodeur JSON
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Log le corps de la requête
	log.Printf("Queue creation request body: %s", string(bodyBytes))

	var request struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"` // Utilisez RawMessage pour éviter les problèmes de décodage
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding request JSON: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Maintenant, décodez la configuration manuellement
	var configMap map[string]any
	if err := json.Unmarshal(request.Config, &configMap); err != nil {
		log.Printf("Error decoding config: %v", err)
		http.Error(w, "Invalid config format", http.StatusBadRequest)
		return
	}

	// Construire la configuration de base
	config := &model.QueueConfig{}

	// Appliquer les valeurs de la requête
	if isPersistent, ok := configMap["isPersistent"].(bool); ok {
		config.IsPersistent = isPersistent
	}

	// JSON envoie les nombres comme float64
	if maxSize, ok := configMap["maxSize"].(float64); ok {
		config.MaxSize = int(maxSize)
	}

	if ttlStr, ok := configMap["ttl"].(string); ok {
		ttl, err := time.ParseDuration(ttlStr)
		if err != nil {
			log.Printf("Error parsing TTL duration: %v", err)
			// Ne pas échouer, utiliser la valeur par défaut
		} else {
			config.TTL = ttl
		}
	}

	// Traiter le mode de livraison
	if modeStr, ok := configMap["deliveryMode"].(string); ok {
		switch modeStr {
		case "broadcast":
			config.DeliveryMode = model.BroadcastMode
		case "roundRobin":
			config.DeliveryMode = model.RoundRobinMode
		case "singleConsumer":
			config.DeliveryMode = model.SingleConsumerMode
		default:
			log.Printf("Unknown delivery mode: %s, using default", modeStr)
			config.DeliveryMode = model.BroadcastMode
		}
	}

	log.Printf("Creating queue with config: %+v", config)

	// Traiter la configuration de retry
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

	// Traiter la configuration du circuit breaker
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
		log.Printf("Error from service: %v", err)
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

// getQueue récupère les détails d'une file d'attente
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

// deleteQueue supprime une file d'attente
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

// publishMessage publie un message dans une file d'attente
func (h *Handler) publishMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]

	// Lire le corps de la requête
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Message payload: %+v", payload)

	// Convertir le payload en JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling payload: %v", err)
		http.Error(w, "Failed to encode payload", http.StatusInternalServerError)
		return
	}

	_, err = h.queueService.GetQueue(r.Context(), domainName, queueName)
	if err != nil {
		log.Printf("Error retrieving queue '%s': %v", queueName, err)
		http.Error(w, fmt.Sprintf("Queue not found: %s", err), http.StatusNotFound)
		return
	}

	// Créer le message
	message := &model.Message{
		ID:        GenerateID(),
		Payload:   payloadBytes,
		Headers:   extractHeaders(r),
		Timestamp: time.Now(),
	}

	// Publier le message
	if err := h.messageService.PublishMessage(domainName, queueName, message); err != nil {
		log.Printf("Error publishing message: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "success",
		"messageId": message.ID,
	})
}

// consumeMessages consomme des messages d'une file d'attente
func (h *Handler) consumeMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]

	// Paramètres optionnels
	timeoutStr := r.URL.Query().Get("timeout")
	maxCountStr := r.URL.Query().Get("max")

	timeout := 0
	if timeoutStr != "" {
		timeout, _ = strconv.Atoi(timeoutStr)
	}

	maxCount := 1
	if maxCountStr != "" {
		maxCount, _ = strconv.Atoi(maxCountStr)
	}

	// Adapter pour le polling long si un timeout est spécifié
	ctx := r.Context()
	if timeout > 0 {
		var cancel context.CancelFunc
		_, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	// Récupérer les messages
	messages := make([]*model.Message, 0, maxCount)
	for i := 0; i < maxCount; i++ {
		message, err := h.messageService.ConsumeMessage(domainName, queueName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if message == nil {
			break // Plus de messages disponibles
		}

		messages = append(messages, message)
	}

	// Convertir les messages pour la réponse
	responseMessages := make([]map[string]any, len(messages))
	for i, msg := range messages {
		// Décoder le payload
		var payload map[string]any
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			payload = map[string]any{"data": string(msg.Payload)}
		}

		// Ajouter les métadonnées
		responseMsg := map[string]any{
			"id":        msg.ID,
			"timestamp": msg.Timestamp,
			"headers":   msg.Headers,
		}

		// Fusionner avec le payload
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

// subscribeToQueue s'abonne à une file d'attente (HTTP API pour registering)
func (h *Handler) subscribeToQueue(w http.ResponseWriter, r *http.Request) {
	// Cette méthode est juste pour l'API HTTP, les vraies souscriptions
	// se font via WebSocket/autres protocoles

	// vars := mux.Vars(r)
	// domainName := vars["domain"]
	// queueName := vars["queue"]

	var request struct {
		CallbackURL string `json:"callbackUrl,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		// Ignorer les erreurs de parsing, c'est peut-être un body vide
	}

	// Générer un ID de souscription (mais pas d'abonnement réel)
	subscriptionID := GenerateID()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":         "success",
		"subscriptionId": subscriptionID,
		"message":        "Use WebSocket for real-time messages",
	})
}

// unsubscribeFromQueue se désinscrit d'une file d'attente
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

// listRoutingRules liste toutes les règles de routage d'un domaine
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

// addRoutingRule ajoute une règle de routage
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

// removeRoutingRule supprime une règle de routage
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

	// Récupérer les statistiques
	stats, err := h.statsService.GetStats(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Écrire la réponse JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// extractHeaders extrait les en-têtes pertinents de la requête
func extractHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)

	// En-têtes pertinents
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

// getCurrentResourceStats retourne les statistiques actuelles d'utilisation des ressources
func (h *Handler) getCurrentResourceStats(w http.ResponseWriter, r *http.Request) {
	if h.resourceMonitor == nil {
		http.Error(w, "Resource monitoring not available", http.StatusServiceUnavailable)
		return
	}

	stats, err := h.resourceMonitor.GetCurrentStats(r.Context())
	if err != nil {
		log.Printf("Error getting current resource stats: %v", err)
		http.Error(w, "Failed to get resource statistics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// getResourceStatsHistory retourne l'historique des statistiques
func (h *Handler) getResourceStatsHistory(w http.ResponseWriter, r *http.Request) {
	if h.resourceMonitor == nil {
		http.Error(w, "Resource monitoring not available", http.StatusServiceUnavailable)
		return
	}

	// Paramètre optionnel pour limiter le nombre de points retournés
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
		log.Printf("Error getting resource stats history: %v", err)
		http.Error(w, "Failed to get resource statistics history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// getDomainResourceStats retourne les statistiques pour un domaine spécifique
func (h *Handler) getDomainResourceStats(w http.ResponseWriter, r *http.Request) {
	if h.resourceMonitor == nil {
		http.Error(w, "Resource monitoring not available", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	domainName := vars["domain"]

	stats, err := h.resourceMonitor.GetCurrentStats(r.Context())
	if err != nil {
		log.Printf("Error getting current resource stats: %v", err)
		http.Error(w, "Failed to get resource statistics", http.StatusInternalServerError)
		return
	}

	// Vérifier si le domaine existe
	domainStats, exists := stats.DomainStats[domainName]
	if !exists {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domainStats)
}

// GenerateID génère un ID unique
func GenerateID() string {
	// Implémentation simple basée sur le timestamp et un nombre aléatoire
	return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}
