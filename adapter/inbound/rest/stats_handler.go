package rest

import (
	"encoding/json"
	"net/http"
)

// handleGetStats récupère les statistiques du système
func (h *Handler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.statsService.GetStats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
