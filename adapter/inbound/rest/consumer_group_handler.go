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

func (h *Handler) getConsumerGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]
	groupID := vars["group"]

	log.Printf("[DEBUG] Getting consumer group details for %s.%s.%s", domainName, queueName, groupID)

	group, err := h.consumerGroupService.GetGroupDetails(r.Context(), domainName, queueName, groupID)
	if err != nil {
		log.Printf("[ERROR] getting consumer group %s.%s.%s: %v", domainName, queueName, groupID, err)

		// Filter error types
		if err.Error() == "consumer group not found" {
			http.Error(w, "Consumer group not found or expired", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Activity update
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

func (h *Handler) createConsumerGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]

	var request struct {
		GroupID string `json:"groupID"`
		TTL     string `json:"ttl,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.GroupID == "" {
		http.Error(w, "GroupID is required", http.StatusBadRequest)
		return
	}

	// Convert TTL to duration
	var ttl time.Duration
	var err error
	if request.TTL != "" && request.TTL != "0" {
		ttl, err = time.ParseDuration(request.TTL)
		if err != nil {
			http.Error(w, "Invalid TTL format", http.StatusBadRequest)
			return
		}
	}

	// create
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

// TODO: check
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

func (h *Handler) updateConsumerGroupTTL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]
	groupID := vars["group"]

	log.Printf("[DEBUG] Updating TTL for consumer group %s.%s.%s", domainName, queueName, groupID)

	var request struct {
		TTL string `json:"ttl"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("[ERROR] Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TTL to duration
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

	// group check
	_, err = h.consumerGroupService.GetGroupDetails(r.Context(), domainName, queueName, groupID)
	if err != nil {
		log.Printf("[ERROR] getting consumer group: %v", err)
		http.Error(w, "Consumer group not found or error: "+err.Error(), http.StatusNotFound)
		return
	}

	// TTL update
	if err := h.consumerGroupService.UpdateConsumerGroupTTL(r.Context(), domainName, queueName, groupID, ttl); err != nil {
		log.Printf("[ERROR] updating TTL: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// update through interface
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

func (h *Handler) getPendingMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]
	groupID := vars["group"]

	log.Printf("API request: Getting pending messages for group %s.%s.%s", domainName, queueName, groupID)

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

func (h *Handler) addConsumerToGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]
	groupID := vars["group"]

	var request struct {
		ConsumerID string `json:"consumerID"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Add consumer
	if err := h.consumerGroupRepo.RegisterConsumer(r.Context(), domainName, queueName, groupID, request.ConsumerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

func (h *Handler) removeConsumerFromGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]
	queueName := vars["queue"]
	groupID := vars["group"]
	consumerID := vars["consumer"]

	// Delete consumer
	if err := h.consumerGroupRepo.RemoveConsumer(r.Context(), domainName, queueName, groupID, consumerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}
