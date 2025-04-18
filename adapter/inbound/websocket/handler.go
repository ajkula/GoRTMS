package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/gorilla/websocket"
)

// Handler gère les connexions WebSocket
type Handler struct {
	messageService inbound.MessageService
	upgrader       websocket.Upgrader
	connections    map[string][]*websocketConnection
	mu             sync.RWMutex
	rootCtx        context.Context
}

// websocketConnection représente une connexion WebSocket active
type websocketConnection struct {
	conn           *websocket.Conn
	domainName     string
	queueName      string
	subscriptionID string
}

// NewHandler crée un nouveau gestionnaire WebSocket
func NewHandler(messageService inbound.MessageService, rootCtx context.Context) *Handler {
	return &Handler{
		messageService: messageService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // À remplacer par une vérification d'origine
			},
		},
		connections: make(map[string][]*websocketConnection),
		rootCtx:     rootCtx,
	}
}

// HandleConnection gère une connexion WebSocket entrante
func (h *Handler) HandleConnection(w http.ResponseWriter, r *http.Request, domainName, queueName string) {
	// Établir la connexion WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		return
	}

	// Créer une clé pour cette file d'attente
	queueKey := domainName + ":" + queueName

	// Créer la connexion
	wsConn := &websocketConnection{
		conn:       conn,
		domainName: domainName,
		queueName:  queueName,
	}

	// Enregistrer la connexion
	h.mu.Lock()
	if _, exists := h.connections[queueKey]; !exists {
		h.connections[queueKey] = make([]*websocketConnection, 0)
	}
	h.connections[queueKey] = append(h.connections[queueKey], wsConn)
	h.mu.Unlock()

	// Configurer l'abonnement à la file d'attente
	subID, err := h.messageService.SubscribeToQueue(
		domainName,
		queueName,
		func(msg *model.Message) error {
			return h.sendMessageToClient(wsConn, msg)
		},
	)

	if err != nil {
		log.Printf("Error subscribing to queue: %v", err)
		conn.Close()
		return
	}

	// Stocker l'ID d'abonnement
	wsConn.subscriptionID = subID

	// Envoyer un message de confirmation
	conn.WriteJSON(map[string]string{
		"type":           "connected",
		"subscriptionId": subID,
		"domain":         domainName,
		"queue":          queueName,
	})

	// Gérer la fermeture de la connexion
	go h.handleWebSocketSession(wsConn)
}

// handleWebSocketSession gère une session WebSocket active
func (h *Handler) handleWebSocketSession(wsConn *websocketConnection) {
	defer func() {
		// Se désinscrire de la file d'attente
		err := h.messageService.UnsubscribeFromQueue(
			wsConn.domainName,
			wsConn.queueName,
			wsConn.subscriptionID,
		)
		if err != nil {
			log.Printf("Error unsubscribing: %v", err)
		}

		// Fermer la connexion
		wsConn.conn.Close()

		// Supprimer la connexion de la liste
		queueKey := wsConn.domainName + ":" + wsConn.queueName
		h.mu.Lock()
		conns := h.connections[queueKey]
		for i, c := range conns {
			if c == wsConn {
				h.connections[queueKey] = append(conns[:i], conns[i+1:]...)
				break
			}
		}
		if len(h.connections[queueKey]) == 0 {
			delete(h.connections, queueKey)
		}
		h.mu.Unlock()
	}()

	// Boucle de lecture des messages du client
	for {
		messageType, data, err := wsConn.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Traiter les messages du client
		h.handleClientMessage(wsConn, messageType, data)
	}
}

// handleClientMessage traite les messages envoyés par le client WebSocket
func (h *Handler) handleClientMessage(wsConn *websocketConnection, messageType int, data []byte) {
	// Uniquement pour les messages texte
	if messageType != websocket.TextMessage {
		return
	}

	// Parser le message JSON
	var message map[string]any
	if err := json.Unmarshal(data, &message); err != nil {
		log.Printf("Error parsing client message: %v", err)
		return
	}

	// Récupérer le type de message
	msgType, ok := message["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "ping":
		// Répondre à un ping
		wsConn.conn.WriteJSON(map[string]string{
			"type": "pong",
		})
	case "publish":
		// Publier un message dans la file d'attente
		payload, ok := message["payload"]
		if !ok {
			return
		}

		// Convertir le payload en JSON
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Error encoding payload: %v", err)
			return
		}

		// Créer le message
		msg := &model.Message{
			ID:        GenerateID(),
			Payload:   payloadBytes,
			Headers:   make(map[string]string),
			Timestamp: time.Now(),
		}

		// Publier le message
		err = h.messageService.PublishMessage(
			wsConn.domainName,
			wsConn.queueName,
			msg,
		)

		if err != nil {
			log.Printf("Error publishing message: %v", err)
			wsConn.conn.WriteJSON(map[string]string{
				"type":  "error",
				"error": err.Error(),
			})
			return
		}

		// Confirmer la publication
		wsConn.conn.WriteJSON(map[string]string{
			"type":      "published",
			"messageId": msg.ID,
		})
	}
}

// sendMessageToClient envoie un message à un client WebSocket
func (h *Handler) sendMessageToClient(wsConn *websocketConnection, msg *model.Message) error {
	// Décoder le payload
	var payload map[string]any
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		payload = map[string]any{
			"data": string(msg.Payload),
		}
	}

	// Créer le message à envoyer
	message := map[string]any{
		"type":      "message",
		"id":        msg.ID,
		"timestamp": msg.Timestamp,
		"headers":   msg.Headers,
		"payload":   payload,
	}

	// Envoyer au client
	return wsConn.conn.WriteJSON(message)
}

// GenerateID génère un ID unique
func GenerateID() string {
	// Implémentation simple basée sur le timestamp et un nombre aléatoire
	return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}

func (h *Handler) Cleanup() {
	log.Println("Cleaning up WebSocket handler resources...")

	// Utiliser un verrou pour éviter les modificaitons concurrentes
	h.mu.Lock()
	defer h.mu.Unlock()

	// Fermer proprement toutes les connexions WebSocket
	for queueKey, connections := range h.connections {
		for _, conn := range connections {
			// Envoyer un message de fermeture
			conn.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Server shutting down"))

			// Fermer la connexion
			conn.conn.Close()

			// Se désabonner
			if conn.subscriptionID != "" {
				h.messageService.UnsubscribeFromQueue(
					conn.domainName,
					conn.queueName,
					conn.subscriptionID,
				)
			}
		}
		delete(h.connections, queueKey)
	}

	log.Println("WebSocket handler cleanup complete")
}
