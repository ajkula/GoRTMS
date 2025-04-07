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
	messageService inbound.MessageService
	domainService  inbound.DomainService
	queueService   inbound.QueueService
	routingService inbound.RoutingService
	statsService   inbound.StatsService
}

// NewHandler crée un nouveau gestionnaire REST
func NewHandler(
	messageService inbound.MessageService,
	domainService inbound.DomainService,
	queueService inbound.QueueService,
	routingService inbound.RoutingService,
	statsService inbound.StatsService,
) *Handler {
	return &Handler{
		messageService: messageService,
		domainService:  domainService,
		queueService:   queueService,
		routingService: routingService,
		statsService:   statsService,
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

	// Route pour les stats
	router.HandleFunc("/api/stats", h.getStats).Methods("GET")

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
	json.NewEncoder(w).Encode(map[string]interface{}{
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
		Name         string `json:"name"`
		MessageCount int    `json:"messageCount"`
	}

	type RouteInfo struct {
		SourceQueue      string      `json:"sourceQueue"`
		DestinationQueue string      `json:"destinationQueue"`
		Predicate        interface{} `json:"predicate"`
	}

	type DomainResponse struct {
		Name   string      `json:"name"`
		Schema interface{} `json:"schema,omitempty"`
		Queues []QueueInfo `json:"queues"`
		Routes []RouteInfo `json:"routes"`
	}

	// Remplir la réponse
	response := DomainResponse{
		Name:   domain.Name,
		Schema: domain.Schema,
		Queues: make([]QueueInfo, 0, len(domain.Queues)),
		Routes: make([]RouteInfo, 0),
	}

	// Ajouter les queues
	for _, queue := range domain.Queues {
		response.Queues = append(response.Queues, QueueInfo{
			Name:         queue.Name,
			MessageCount: queue.MessageCount,
		})
	}

	// Ajouter les routes
	for srcQueue, dstRoutes := range domain.Routes {
		for dstQueue, rule := range dstRoutes {
			response.Routes = append(response.Routes, RouteInfo{
				SourceQueue:      srcQueue,
				DestinationQueue: dstQueue,
				Predicate:        rule.Predicate,
			})
		}
	}

	// Log la réponse pour débogage
	respBytes, _ := json.MarshalIndent(response, "", "  ")
	log.Printf("Domain response: %s", string(respBytes))

	w.Header().Set("Content-Type", "application/json")
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
		Name         string `json:"name"`
		MessageCount int    `json:"messageCount"`
	}

	response := make([]queueResponse, len(queues))
	for i, queue := range queues {
		response[i] = queueResponse{
			Name:         queue.Name,
			MessageCount: queue.MessageCount,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
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

	log.Printf("Parsed queue name: %s", request.Name)

	// Maintenant, décodez la configuration manuellement
	var configMap map[string]interface{}
	if err := json.Unmarshal(request.Config, &configMap); err != nil {
		log.Printf("Error decoding config: %v", err)
		http.Error(w, "Invalid config format", http.StatusBadRequest)
		return
	}

	// Construire la configuration
	config := &model.QueueConfig{
		IsPersistent: false, // Valeurs par défaut
		MaxSize:      0,
		TTL:          0,
	}

	// Appliquer les valeurs de la requête
	if isPersistent, ok := configMap["isPersistent"].(bool); ok {
		config.IsPersistent = isPersistent
	}

	if maxSize, ok := configMap["maxSize"].(float64); ok { // JSON envoie les nombres comme float64
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

	if err := h.queueService.CreateQueue(r.Context(), domainName, request.Name, config); err != nil {
		log.Printf("Error from service: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"queue":  request.Name,
	})
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
	json.NewEncoder(w).Encode(queue)
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

	log.Printf("Publishing message to domain '%s', queue '%s'", domainName, queueName)

	// Lire le corps de la requête
	var payload map[string]interface{}
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
	if err := h.messageService.PublishMessage(r.Context(), domainName, queueName, message); err != nil {
		log.Printf("Error publishing message: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Message published successfully!")

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
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	// Récupérer les messages
	messages := make([]*model.Message, 0, maxCount)
	for i := 0; i < maxCount; i++ {
		message, err := h.messageService.ConsumeMessage(ctx, domainName, queueName)
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
	responseMessages := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		// Décoder le payload
		var payload map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			payload = map[string]interface{}{"data": string(msg.Payload)}
		}

		// Ajouter les métadonnées
		responseMsg := map[string]interface{}{
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
	json.NewEncoder(w).Encode(map[string]interface{}{
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

	if err := h.messageService.UnsubscribeFromQueue(r.Context(), domainName, queueName, request.SubscriptionID); err != nil {
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
	json.NewEncoder(w).Encode(map[string]interface{}{
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

// GenerateID génère un ID unique
func GenerateID() string {
	// Implémentation simple basée sur le timestamp et un nombre aléatoire
	return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}
