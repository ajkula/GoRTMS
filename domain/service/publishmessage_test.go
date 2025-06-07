package service

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test evaluateJSONPredicate - méthode pure sans dépendances
func TestEvaluateJSONPredicate(t *testing.T) {
	service := &MessageServiceImpl{
		logger: &mockLogger{},
	}

	// Helper to create message with JSON payload
	createMessage := func(payload string) *model.Message {
		return &model.Message{
			ID:        "test-msg",
			Payload:   []byte(payload),
			Timestamp: time.Now(),
		}
	}

	t.Run("Equals predicate (eq)", func(t *testing.T) {
		predicate := model.JSONPredicate{
			Type:  "eq",
			Field: "status",
			Value: "active",
		}

		// Match case
		message := createMessage(`{"status": "active", "id": 123}`)
		result := service.evaluateJSONPredicate(predicate, message)
		assert.True(t, result)

		// No match case
		message = createMessage(`{"status": "inactive", "id": 123}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)

		// Field missing
		message = createMessage(`{"id": 123}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)
	})

	t.Run("Not equals predicate (ne)", func(t *testing.T) {
		predicate := model.JSONPredicate{
			Type:  "ne",
			Field: "status",
			Value: "deleted",
		}

		// Match case (not deleted)
		message := createMessage(`{"status": "active"}`)
		result := service.evaluateJSONPredicate(predicate, message)
		assert.True(t, result)

		// No match case (is deleted)
		message = createMessage(`{"status": "deleted"}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)
	})

	t.Run("Greater than predicate (gt)", func(t *testing.T) {
		predicate := model.JSONPredicate{
			Type:  "gt",
			Field: "priority",
			Value: 5.0,
		}

		// Match case
		message := createMessage(`{"priority": 10.0}`)
		result := service.evaluateJSONPredicate(predicate, message)
		assert.True(t, result)

		// No match case
		message = createMessage(`{"priority": 3.0}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)

		// Equal case (should be false for gt)
		message = createMessage(`{"priority": 5.0}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)

		// Non-numeric field
		message = createMessage(`{"priority": "high"}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)
	})

	t.Run("Less than predicate (lt)", func(t *testing.T) {
		predicate := model.JSONPredicate{
			Type:  "lt",
			Field: "score",
			Value: 100.0,
		}

		// Match case
		message := createMessage(`{"score": 50.0}`)
		result := service.evaluateJSONPredicate(predicate, message)
		assert.True(t, result)

		// No match case
		message = createMessage(`{"score": 150.0}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)

		// Equal case (should be false for lt)
		message = createMessage(`{"score": 100.0}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)
	})

	t.Run("Contains predicate", func(t *testing.T) {
		predicate := model.JSONPredicate{
			Type:  "contains",
			Field: "description",
			Value: "urgent",
		}

		// Match case
		message := createMessage(`{"description": "This is urgent business"}`)
		result := service.evaluateJSONPredicate(predicate, message)
		assert.True(t, result)

		// No match case
		message = createMessage(`{"description": "Normal priority task"}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)

		// Case sensitive
		message = createMessage(`{"description": "This is URGENT business"}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)

		// Non-string field
		message = createMessage(`{"description": 123}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)
	})

	t.Run("Invalid JSON payload", func(t *testing.T) {
		predicate := model.JSONPredicate{
			Type:  "eq",
			Field: "status",
			Value: "active",
		}

		message := &model.Message{
			ID:      "test-msg",
			Payload: []byte(`{invalid json`),
		}

		result := service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)
	})

	t.Run("Unknown predicate type", func(t *testing.T) {
		predicate := model.JSONPredicate{
			Type:  "unknown_type",
			Field: "status",
			Value: "active",
		}

		message := createMessage(`{"status": "active"}`)
		result := service.evaluateJSONPredicate(predicate, message)
		assert.False(t, result)
	})

	t.Run("Edge cases with different data types", func(t *testing.T) {
		// Numeric comparison with different types
		predicate := model.JSONPredicate{
			Type:  "gt",
			Field: "value",
			Value: 10.0,
		}

		// Integer in JSON becomes float64
		message := createMessage(`{"value": 15}`)
		result := service.evaluateJSONPredicate(predicate, message)
		assert.True(t, result)

		// String equals with numeric data
		predicate = model.JSONPredicate{
			Type:  "eq",
			Field: "id",
			Value: "123",
		}

		message = createMessage(`{"id": 123}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.True(t, result) // String conversion should work

		// Boolean comparison
		predicate = model.JSONPredicate{
			Type:  "eq",
			Field: "active",
			Value: true,
		}

		message = createMessage(`{"active": true}`)
		result = service.evaluateJSONPredicate(predicate, message)
		assert.True(t, result)
	})
}

// Test schema validation logic - partie isolée de PublishMessage
func TestMessageValidation(t *testing.T) {
	t.Run("Field type validation", func(t *testing.T) {
		schema := &model.Schema{
			Fields: map[string]model.FieldType{
				"name":   model.StringType,
				"age":    model.NumberType,
				"active": model.BooleanType,
			},
		}

		tests := []struct {
			name        string
			payload     string
			shouldPass  bool
			description string
		}{
			{
				name:        "Valid all types",
				payload:     `{"name": "John", "age": 30, "active": true}`,
				shouldPass:  true,
				description: "All fields with correct types",
			},
			{
				name:        "Missing field",
				payload:     `{"name": "John", "age": 30}`,
				shouldPass:  false,
				description: "Missing active field",
			},
			{
				name:        "Wrong string type",
				payload:     `{"name": 123, "age": 30, "active": true}`,
				shouldPass:  false,
				description: "Name should be string",
			},
			{
				name:        "Wrong number type",
				payload:     `{"name": "John", "age": "thirty", "active": true}`,
				shouldPass:  false,
				description: "Age should be number",
			},
			{
				name:        "Wrong boolean type",
				payload:     `{"name": "John", "age": 30, "active": "yes"}`,
				shouldPass:  false,
				description: "Active should be boolean",
			},
			{
				name:        "Extra fields allowed",
				payload:     `{"name": "John", "age": 30, "active": true, "extra": "field"}`,
				shouldPass:  true,
				description: "Extra fields should be allowed",
			},
			{
				name:        "Invalid JSON",
				payload:     `{invalid json}`,
				shouldPass:  false,
				description: "Malformed JSON should fail",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validateMessageSchema([]byte(tt.payload), schema)
				if tt.shouldPass {
					assert.NoError(t, err, tt.description)
				} else {
					assert.Error(t, err, tt.description)
				}
			})
		}
	})

	t.Run("Custom validation function", func(t *testing.T) {
		customValidation := func(payload []byte) error {
			var data map[string]interface{}
			if err := json.Unmarshal(payload, &data); err != nil {
				return err
			}

			// Business rule: if priority > 5, description is required
			if priority, ok := data["priority"].(float64); ok && priority > 5 {
				if desc, exists := data["description"]; !exists || desc == "" {
					return errors.New("description required for high priority")
				}
			}
			return nil
		}

		schema := &model.Schema{
			Validation: customValidation,
		}

		tests := []struct {
			name       string
			payload    string
			shouldPass bool
		}{
			{
				name:       "Low priority without description",
				payload:    `{"priority": 3}`,
				shouldPass: true,
			},
			{
				name:       "High priority with description",
				payload:    `{"priority": 8, "description": "Important task"}`,
				shouldPass: true,
			},
			{
				name:       "High priority without description",
				payload:    `{"priority": 8}`,
				shouldPass: false,
			},
			{
				name:       "High priority with empty description",
				payload:    `{"priority": 8, "description": ""}`,
				shouldPass: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validateMessageSchema([]byte(tt.payload), schema)
				if tt.shouldPass {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			})
		}
	})
}

// Helper function extracted from PublishMessage logic
func validateMessageSchema(payload []byte, schema *model.Schema) error {
	if schema == nil {
		return nil
	}

	// Custom validation takes precedence
	if schema.Validation != nil {
		return schema.Validation(payload)
	}

	// Field validation
	if len(schema.Fields) > 0 {
		var data map[string]interface{}
		if err := json.Unmarshal(payload, &data); err != nil {
			return err
		}

		for fieldName, fieldType := range schema.Fields {
			fieldValue, exists := data[fieldName]
			if !exists {
				return errors.New("missing required field: " + fieldName)
			}

			switch fieldType {
			case model.StringType:
				if _, ok := fieldValue.(string); !ok {
					return errors.New("field " + fieldName + " should be string")
				}
			case model.NumberType:
				if _, ok := fieldValue.(float64); !ok {
					return errors.New("field " + fieldName + " should be number")
				}
			case model.BooleanType:
				if _, ok := fieldValue.(bool); !ok {
					return errors.New("field " + fieldName + " should be boolean")
				}
			}
		}
	}

	return nil
}

// Test metadata enrichment logic
func TestMessageMetadataEnrichment(t *testing.T) {
	t.Run("Add metadata to nil metadata", func(t *testing.T) {
		message := &model.Message{
			ID:       "test-123",
			Payload:  []byte(`{"content": "test"}`),
			Metadata: nil,
		}

		enrichMessageMetadata(message, "test-domain", "test-queue")

		require.NotNil(t, message.Metadata)
		assert.Equal(t, "test-domain", message.Metadata["domain"])
		assert.Equal(t, "test-queue", message.Metadata["queue"])
	})

	t.Run("Preserve existing metadata", func(t *testing.T) {
		message := &model.Message{
			ID:      "test-123",
			Payload: []byte(`{"content": "test"}`),
			Metadata: map[string]interface{}{
				"source":   "api",
				"priority": 5,
			},
		}

		enrichMessageMetadata(message, "test-domain", "test-queue")

		assert.Equal(t, "test-domain", message.Metadata["domain"])
		assert.Equal(t, "test-queue", message.Metadata["queue"])
		assert.Equal(t, "api", message.Metadata["source"])
		assert.Equal(t, 5, message.Metadata["priority"])
	})

	t.Run("Set timestamp if zero", func(t *testing.T) {
		message := &model.Message{
			ID:        "test-123",
			Payload:   []byte(`{"content": "test"}`),
			Timestamp: time.Time{},
		}

		before := time.Now()
		enrichMessageTimestamp(message)
		after := time.Now()

		assert.False(t, message.Timestamp.IsZero())
		assert.True(t, message.Timestamp.After(before) || message.Timestamp.Equal(before))
		assert.True(t, message.Timestamp.Before(after) || message.Timestamp.Equal(after))
	})

	t.Run("Preserve existing timestamp", func(t *testing.T) {
		existingTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		message := &model.Message{
			ID:        "test-123",
			Payload:   []byte(`{"content": "test"}`),
			Timestamp: existingTime,
		}

		enrichMessageTimestamp(message)

		assert.Equal(t, existingTime, message.Timestamp)
	})
}

// Helper functions extracted from MessageService logic
func enrichMessageMetadata(message *model.Message, domainName, queueName string) {
	if message.Metadata == nil {
		message.Metadata = make(map[string]interface{})
	}
	message.Metadata["domain"] = domainName
	message.Metadata["queue"] = queueName
}

func enrichMessageTimestamp(message *model.Message) {
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}
}
