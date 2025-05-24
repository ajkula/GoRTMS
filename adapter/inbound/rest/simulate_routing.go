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

func (h *Handler) testRoutingRules(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["domain"]

	var request struct {
		Queue   string                 `json:"queue"`   // Source queue
		Payload map[string]interface{} `json:"payload"` // Test message content
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding test routing request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Source Q exists check
	_, err := h.queueService.GetQueue(r.Context(), domainName, request.Queue)
	if err != nil {
		http.Error(w, fmt.Sprintf("Source queue not found: %s", err), http.StatusNotFound)
		return
	}

	// Payload to JSON
	payloadBytes, err := json.Marshal(request.Payload)
	if err != nil {
		http.Error(w, "Failed to encode payload", http.StatusInternalServerError)
		return
	}

	id := "test-" + GenerateID()
	if val, ok := request.Payload["id"]; ok {
		if str, ok := val.(string); ok && str != "" {
			id = str
		}
	}

	// Create test msg
	testMessage := &model.Message{
		ID:        id,
		Payload:   payloadBytes,
		Headers:   make(map[string]string),
		Timestamp: time.Now(),
	}

	// Get all routing rules for the domain
	rules, err := h.routingService.ListRoutingRules(r.Context(), domainName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter the rules that have the specified queue as the source
	sourceRules := make([]*model.RoutingRule, 0)
	for _, rule := range rules {
		if rule.SourceQueue == request.Queue {
			sourceRules = append(sourceRules, rule)
		}
	}

	// Test each rule
	type MatchResult struct {
		Rule             *model.RoutingRule `json:"rule"`
		Matches          bool               `json:"matches"`
		DestinationQueue string             `json:"destinationQueue"`
	}

	matches := make([]MatchResult, 0, len(sourceRules))
	for _, rule := range sourceRules {
		// Evaluate the predicate to see if the rule applies
		isMatch := evaluatePredicate(rule.Predicate, testMessage)

		matches = append(matches, MatchResult{
			Rule:             rule,
			Matches:          isMatch,
			DestinationQueue: rule.DestinationQueue,
		})
	}

	response := map[string]interface{}{
		"sourceQueue": request.Queue,
		"messageId":   testMessage.ID,
		"matches":     matches,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func evaluatePredicate(predicate interface{}, message *model.Message) bool {
	// if JSON
	if jsonPred, ok := predicate.(model.JSONPredicate); ok {
		return evaluateJSONPredicate(jsonPred, message)
	}

	// if map
	if mapPred, ok := predicate.(map[string]interface{}); ok {
		jsonPred := model.JSONPredicate{
			Type:  mapPred["type"].(string),
			Field: mapPred["field"].(string),
			Value: mapPred["value"],
		}
		return evaluateJSONPredicate(jsonPred, message)
	}

	// if func
	if predFunc, ok := predicate.(model.PredicateFunc); ok {
		return predFunc(message)
	}

	log.Printf("Unsupported predicate type: %T", predicate)
	return false
}

func evaluateJSONPredicate(predicate model.JSONPredicate, message *model.Message) bool {

	// decode payload
	var payload map[string]interface{}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		log.Printf("Error decoding message payload for predicate evaluation: %v", err)
		return false
	}

	fieldPath := strings.Split(predicate.Field, ".")
	fieldValue := getNestedValue(payload, fieldPath)

	if fieldValue == nil {
		return false
	}

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

// extracts a nested value from a map
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

// Helper functions for comparisons
func isEqual(a, b interface{}) bool {
	// Basic implementation, to be improved to handle different types
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func isGreaterThan(a, b interface{}) bool {
	// Convert to numbers if possible
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)

	if aOk && bOk {
		return aFloat > bFloat
	}

	// Lexicographic comparison for strings
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

// toFloat64 tries to convert a value to float64
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
