package rest

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
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

	// Note: Cette fonctionnalité nécessite une méthode pour récupérer les messages
	// en attente pour un groupe spécifique. L'implémentation dépend de l'architecture existante.
	// Voici un exemple simplifié:

	// Supposons qu'on ait accès à une méthode pour obtenir les messages en attente
	// Ceci est un placeholder, à adapter selon votre architecture
	messages, err := h.getPendingMessagesForGroup(r.Context(), domainName, queueName, groupID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"messages": messages,
	})
}

// getPendingMessagesForGroup est une méthode helper pour récupérer les messages en attente
// À adapter selon votre architecture existante
func (h *Handler) getPendingMessagesForGroup(ctx context.Context, domainName, queueName, groupID string) ([]*model.Message, error) {
	// Placeholder - à implémenter selon votre architecture
	// Cette implémentation dépend de comment les messages sont stockés et comment
	// vous pouvez déterminer quels messages sont en attente pour un groupe spécifique.
	return []*model.Message{}, nil
}

/**
// Implémenter correctement getPendingMessagesForGroup pour récupérer les vrais messages
func (h *Handler) getPendingMessagesForGroup(ctx context.Context, domainName, queueName, groupID string) ([]*model.Message, error) {
    log.Printf("Getting pending messages for group %s.%s.%s", domainName, queueName, groupID)

    // Utiliser la matrice d'acquittement pour trouver les messages en attente
    matrix := h.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
    if matrix == nil {
        log.Printf("No acknowledgment matrix found for %s.%s", domainName, queueName)
        return []*model.Message{}, nil
    }

    // Récupérer les IDs des messages en attente
    pendingIDs := matrix.GetPendingMessageIDs(groupID)
    if len(pendingIDs) == 0 {
        log.Printf("No pending message IDs found for group %s", groupID)
        return []*model.Message{}, nil
    }

    log.Printf("Found %d pending message IDs for group %s", len(pendingIDs), groupID)

    // Récupérer les messages correspondants
    messages := make([]*model.Message, 0, len(pendingIDs))
    for _, msgID := range pendingIDs {
        msg, err := h.messageRepo.GetMessage(ctx, domainName, queueName, msgID)
        if err != nil {
            log.Printf("Warning: Could not retrieve message %s: %v", msgID, err)
            continue
        }
        messages = append(messages, msg)
    }

    log.Printf("Returning %d pending messages for group %s", len(messages), groupID)
    return messages, nil
}*/

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
