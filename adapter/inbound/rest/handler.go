package rest

import (
	"context"
	"encoding/json"
	"fmt"
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

// Middleware pour servir index.html dans les dossiers
func serveIndexMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Si l'URL se termine par /, servir index.html
		if strings.HasSuffix(path, "/") {
			http.ServeFile(w, r, filepath.Join("./web/dist", path, "index.html"))
			return
		}
		h.ServeHTTP(w, r)
	})
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
	router.HandleFunc("/api/stats", h.handleGetStats).Methods("GET")

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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domain)
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

	var request struct {
		Name   string            `json:"name"`
		Config model.QueueConfig `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.queueService.CreateQueue(r.Context(), domainName, request.Name, &request.Config); err != nil {
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

	// Lire le corps de la requête
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Convertir le payload en JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Failed to encode payload", http.StatusInternalServerError)
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
