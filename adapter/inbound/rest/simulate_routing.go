package rest

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/gorilla/mux"
)

// testRoutingRules teste si un message serait routé selon les règles actuelles
func (h *Handler) testRoutingRules(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]

	// Analyser le corps de la requête
	var request struct {
		Queue   string                 `json:"queue"`   // File d'attente source
		Payload map[string]interface{} `json:"payload"` // Contenu du message de test
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding test routing request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Vérifier que la file source existe
	_, err := h.queueService.GetQueue(r.Context(), domainName, request.Queue)
	if err != nil {
		http.Error(w, fmt.Sprintf("Source queue not found: %s", err), http.StatusNotFound)
		return
	}

	// Convertir le payload en JSON
	payloadBytes, err := json.Marshal(request.Payload)
	if err != nil {
		http.Error(w, "Failed to encode payload", http.StatusInternalServerError)
		return
	}

	// Créer un message de test
	testMessage := &model.Message{
		ID:        "test-" + GenerateID(),
		Payload:   payloadBytes,
		Headers:   make(map[string]string),
		Timestamp: time.Now(),
	}

	// Obtenir toutes les règles de routage pour le domaine
	rules, err := h.routingService.ListRoutingRules(r.Context(), domainName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filtrer les règles qui ont la file spécifiée comme source
	sourceRules := make([]*model.RoutingRule, 0)
	for _, rule := range rules {
		if rule.SourceQueue == request.Queue {
			sourceRules = append(sourceRules, rule)
		}
	}

	// Tester chaque règle
	type MatchResult struct {
		Rule             *model.RoutingRule `json:"rule"`
		Matches          bool               `json:"matches"`
		DestinationQueue string             `json:"destinationQueue"`
	}

	matches := make([]MatchResult, 0, len(sourceRules))
	for _, rule := range sourceRules {
		// Évaluer le prédicat pour voir si la règle s'applique
		isMatch := evaluatePredicate(rule.Predicate, testMessage)

		matches = append(matches, MatchResult{
			Rule:             rule,
			Matches:          isMatch,
			DestinationQueue: rule.DestinationQueue,
		})
	}

	// Générer la réponse
	response := map[string]interface{}{
		"sourceQueue": request.Queue,
		"messageId":   testMessage.ID,
		"matches":     matches,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// evaluatePredicate évalue un prédicat sur un message
func evaluatePredicate(predicate interface{}, message *model.Message) bool {
	// Cas où nous avons un prédicat JSON
	if jsonPred, ok := predicate.(model.JSONPredicate); ok {
		return evaluateJSONPredicate(jsonPred, message)
	}

	// Si le prédicat est déjà un objet JsonPredicate sous forme de map
	if mapPred, ok := predicate.(map[string]interface{}); ok {
		jsonPred := model.JSONPredicate{
			Type:  mapPred["type"].(string),
			Field: mapPred["field"].(string),
			Value: mapPred["value"],
		}
		return evaluateJSONPredicate(jsonPred, message)
	}

	// Si nous avons une fonction de prédicat (cas avancé)
	if predFunc, ok := predicate.(model.PredicateFunc); ok {
		return predFunc(message)
	}

	// Prédicat inconnu ou non supporté
	log.Printf("Unsupported predicate type: %T", predicate)
	return false
}

// evaluateJSONPredicate évalue un prédicat JSON sur un message
func evaluateJSONPredicate(predicate model.JSONPredicate, message *model.Message) bool {
	// Décoder le payload du message
	var payload map[string]interface{}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		log.Printf("Error decoding message payload for predicate evaluation: %v", err)
		return false
	}

	// Obtenir la valeur du champ à partir du payload
	fieldPath := strings.Split(predicate.Field, ".")
	fieldValue := getNestedValue(payload, fieldPath)

	if fieldValue == nil {
		// Le champ n'existe pas dans le message
		return false
	}

	// Comparer selon le type d'opération
	switch predicate.Type {
	case "eq": // equals
		return isEqual(fieldValue, predicate.Value)
	case "neq": // not equals
		return !isEqual(fieldValue, predicate.Value)
	case "gt": // greater than
		return isGreaterThan(fieldValue, predicate.Value)
	case "gte": // greater than or equal
		return isGreaterThanOrEqual(fieldValue, predicate.Value)
	case "lt": // less than
		return isLessThan(fieldValue, predicate.Value)
	case "lte": // less than or equal
		return isLessThanOrEqual(fieldValue, predicate.Value)
	case "contains": // contains substring or element
		return contains(fieldValue, predicate.Value)
	default:
		log.Printf("Unsupported predicate operation: %s", predicate.Type)
		return false
	}
}

// getNestedValue extrait une valeur imbriquée d'une map
func getNestedValue(data map[string]interface{}, path []string) interface{} {
	if len(path) == 0 {
		return nil
	}

	if len(path) == 1 {
		return data[path[0]]
	}

	if nestedData, ok := data[path[0]].(map[string]interface{}); ok {
		return getNestedValue(nestedData, path[1:])
	}

	return nil
}

// Fonctions auxiliaires pour les comparaisons
func isEqual(a, b interface{}) bool {
	// Implémentation de base, à améliorer pour gérer différents types
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func isGreaterThan(a, b interface{}) bool {
	// Convertir en nombres si possible
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)

	if aOk && bOk {
		return aFloat > bFloat
	}

	// Comparaison lexicographique pour les strings
	return fmt.Sprintf("%v", a) > fmt.Sprintf("%v", b)
}

func isGreaterThanOrEqual(a, b interface{}) bool {
	return isEqual(a, b) || isGreaterThan(a, b)
}

func isLessThan(a, b interface{}) bool {
	return !isGreaterThanOrEqual(a, b)
}

func isLessThanOrEqual(a, b interface{}) bool {
	return !isGreaterThan(a, b)
}

func contains(a, b interface{}) bool {
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.Contains(aStr, bStr)
}

// toFloat64 tente de convertir une valeur en float64
func toFloat64(v interface{}) (float64, bool) {
	switch value := v.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case string:
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
