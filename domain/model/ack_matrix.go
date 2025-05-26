package model

import (
	"sync"
)

/*
AckMatrix implements a message acknowledgment tracking system by consumer groups.

High-level architecture:
1. Repository = single source of truth for all messages
2. ChannelQueue = temporary buffer for message delivery
3. AckMatrix = tracks which consumer groups have acknowledged which messages
4. A dual-channel system separates commands from messages

A message is only permanently deleted from the repository once all active
consumer groups have acknowledged it. If a consumer group is removed,
its acknowledgments are automatically considered complete.
*/

// AckMatrix tracks which consumer groups have processed which messages.
type AckMatrix struct {
	mu sync.RWMutex
	// Messages pending acknowledgment (sparse matrix)
	messages map[string]map[string]bool // messageID → (groupID → acknowledged)
	// Groupes de consommateurs actifs
	activeGroups map[string]bool // groupID → active status
	// Total number of active groups
	groupCount int
}

// NewAckMatrix creates a new acknowledgment matrix.
func NewAckMatrix() *AckMatrix {
	return &AckMatrix{
		messages:     make(map[string]map[string]bool),
		activeGroups: make(map[string]bool),
	}
}

// RegisterGroup registers a new consumer group.
func (m *AckMatrix) RegisterGroup(groupID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.activeGroups[groupID] = true
	m.groupCount = len(m.activeGroups)
}

// RemoveGroup removes a consumer group and returns the IDs of messages
// that are now fully acknowledged as a result.
func (m *AckMatrix) RemoveGroup(groupID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.activeGroups, groupID)
	m.groupCount = len(m.activeGroups)

	// Find messages that can now be deleted
	messagesToDelete := []string{}
	for msgID, acks := range m.messages {
		// Mark this group as acknowledged (since it's gone)
		acks[groupID] = true

		// Check if all remaining groups have acknowledged
		allAcked := true
		for g := range m.activeGroups {
			if !acks[g] {
				allAcked = false
				break
			}
		}

		if allAcked {
			messagesToDelete = append(messagesToDelete, msgID)
		}
	}

	// Remove fully acknowledged messages from the matrix
	for _, msgID := range messagesToDelete {
		delete(m.messages, msgID)
	}

	return messagesToDelete
}

// Acknowledge marks a message as acknowledged by a group.
// Returns true if the message is now fully acknowledged.
func (m *AckMatrix) Acknowledge(messageID, groupID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure the group exists
	if !m.activeGroups[groupID] {
		return false
	}

	// Initialize tracking for this message if needed
	if _, exists := m.messages[messageID]; !exists {
		m.messages[messageID] = make(map[string]bool, m.groupCount)
	}

	// Mark as acknowledged
	m.messages[messageID][groupID] = true

	// Check if all groups have acknowledged
	allAcked := true
	for g := range m.activeGroups {
		if !m.messages[messageID][g] {
			allAcked = false
			break
		}
	}

	// Remove from tracking if fully acknowledged
	if allAcked {
		delete(m.messages, messageID)
	}

	return allAcked
}

// Expose the number of currently active groups.
func (m *AckMatrix) GetActiveGroupCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.groupCount
}

// GetPendingMessageCount returns the number of unacknowledged messages for a given group.
func (m *AckMatrix) GetPendingMessageCount(groupID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, acks := range m.messages {
		if !acks[groupID] {
			count++
		}
	}
	return count
}

// GetPendingMessageIDs returns the IDs of messages pending acknowledgment for a given group.
func (m *AckMatrix) GetPendingMessageIDs(groupID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0)
	for id, acks := range m.messages {
		if !acks[groupID] {
			ids = append(ids, id)
		}
	}
	return ids
}
