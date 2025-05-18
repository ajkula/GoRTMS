package rest

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func (h *Handler) listAllConsumerGroups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	groups, err := h.consumerGroupService.ListAllGroups(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	b, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		log.Println("error:", err)
	}
	log.Println(string(b))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"groups": groups,
	})
}

// listConsumerGroups liste tous les consumer groups d'une queue
func (h *Handler) listConsumerGroups(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]

	groups, err := h.consumerGroupService.ListConsumerGroups(r.Context(), domainName, queueName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"groups": groups,
	})
}

// getConsumerGroup récupère les détails d'un consumer group
func (h *Handler) getConsumerGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]
	groupID := vars["group"]

	log.Printf("[DEBUG] Getting consumer group details for %s.%s.%s", domainName, queueName, groupID)

	group, err := h.consumerGroupService.GetGroupDetails(r.Context(), domainName, queueName, groupID)
	if err != nil {
		log.Printf("[ERROR] getting consumer group %s.%s.%s: %v", domainName, queueName, groupID, err)

		// Différencier les différents types d'erreurs
		if err.Error() == "consumer group not found" {
			http.Error(w, "Consumer group not found or expired", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// MAJ explicite de l'activité
	if updater, ok := h.consumerGroupService.(interface {
		UpdateLastActivity(ctx context.Context, domainName, queueName, groupID, consumerID string) error
	}); ok {
		if err := updater.UpdateLastActivity(r.Context(), domainName, queueName, groupID, ""); err != nil {
			log.Printf("Warning: Failed to update last activity: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(group)
}

// createConsumerGroup crée un nouveau consumer group
func (h *Handler) createConsumerGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]

	// Lire le corps de la requête
	var request struct {
		GroupID string `json:"groupID"`
		TTL     string `json:"ttl,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Valider les paramètres
	if request.GroupID == "" {
		http.Error(w, "GroupID is required", http.StatusBadRequest)
		return
	}

	// Convertir le TTL en duration
	var ttl time.Duration
	var err error
	if request.TTL != "" && request.TTL != "0" {
		ttl, err = time.ParseDuration(request.TTL)
		if err != nil {
			http.Error(w, "Invalid TTL format", http.StatusBadRequest)
			return
		}
	}

	// Créer le consumer group
	if err := h.consumerGroupService.CreateConsumerGroup(r.Context(), domainName, queueName, request.GroupID, ttl); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"groupID": request.GroupID,
	})
}

// deleteConsumerGroup supprime un consumer group
func (h *Handler) deleteConsumerGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]
	groupID := vars["group"]

	if err := h.consumerGroupService.DeleteConsumerGroup(r.Context(), domainName, queueName, groupID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

// updateConsumerGroupTTL met à jour le TTL d'un consumer group
func (h *Handler) updateConsumerGroupTTL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]
	groupID := vars["group"]

	log.Printf("[DEBUG] Updating TTL for consumer group %s.%s.%s", domainName, queueName, groupID)

	// Lire le corps de la requête
	var request struct {
		TTL string `json:"ttl"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("[ERROR] Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Convertir le TTL en duration
	var ttl time.Duration
	var err error
	if request.TTL != "" && request.TTL != "0" {
		ttl, err = time.ParseDuration(request.TTL)
		if err != nil {
			log.Printf("[ERROR] Invalid TTL format: %v", err)
			http.Error(w, "Invalid TTL format", http.StatusBadRequest)
			return
		}
	}

	// Vérifier d'abord que le groupe existe
	_, err = h.consumerGroupService.GetGroupDetails(r.Context(), domainName, queueName, groupID)
	if err != nil {
		log.Printf("[ERROR] getting consumer group: %v", err)
		http.Error(w, "Consumer group not found or error: "+err.Error(), http.StatusNotFound)
		return
	}

	// Mettre à jour le TTL
	if err := h.consumerGroupService.UpdateConsumerGroupTTL(r.Context(), domainName, queueName, groupID, ttl); err != nil {
		log.Printf("[ERROR] updating TTL: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Mise à jour explicite de l'activité (si l'interface le prend en charge)
	if updater, ok := h.consumerGroupService.(interface {
		UpdateLastActivity(ctx context.Context, domainName, queueName, groupID, consumerID string) error
	}); ok {
		if err := updater.UpdateLastActivity(r.Context(), domainName, queueName, groupID, ""); err != nil {
			log.Printf("Warning: Failed to update last activity: %v", err)
		}
	}

	log.Printf("[DEBUG] TTL updated successfully for consumer group %s.%s.%s", domainName, queueName, groupID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

// getPendingMessages récupère les messages en attente pour un consumer group
func (h *Handler) getPendingMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]
	groupID := vars["group"]

	log.Printf("API request: Getting pending messages for group %s.%s.%s", domainName, queueName, groupID)

	// Utiliser la méthode du service directement
	messages, err := h.consumerGroupService.GetPendingMessages(r.Context(), domainName, queueName, groupID)
	if err != nil {
		log.Printf("Error getting pending messages: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"messages": messages,
	})
}

// addConsumerToGroup ajoute un consumer à un groupe
func (h *Handler) addConsumerToGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]
	groupID := vars["group"]

	// Lire le corps de la requête
	var request struct {
		ConsumerID string `json:"consumerID"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ajouter le consumer
	if err := h.consumerGroupRepo.RegisterConsumer(r.Context(), domainName, queueName, groupID, request.ConsumerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

// removeConsumerFromGroup supprime un consumer d'un groupe
func (h *Handler) removeConsumerFromGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]
	groupID := vars["group"]
	consumerID := vars["consumer"]

	// Supprimer le consumer
	if err := h.consumerGroupRepo.RemoveConsumer(r.Context(), domainName, queueName, groupID, consumerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}
