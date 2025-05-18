package model

import (
	"sync"
)

/*
AckMatrix implémente un système de suivi des acquittements de messages par groupes de consommateurs.

Architecture générale:
1. Repository = source unique de vérité pour les messages
2. ChannelQueue = buffer temporaire pour la distribution des messages
3. AckMatrix = suivi des messages acquittés par chaque consumer group
4. Le système de double canal sépare les commandes et les messages

Un message n'est définitivement supprimé du repository que lorsque tous les consumer
groups actifs l'ont acquitté. Si un consumer group est supprimé, ses acquittements
sont automatiquement considérés comme complets.
*/

// AckMatrix suit quels consumer groups ont traité quels messages
type AckMatrix struct {
	mu sync.RWMutex
	// Messages en attente d'acquittement (matrice creuse)
	messages map[string]map[string]bool // messageID → (groupID → acquitté)
	// Groupes de consommateurs actifs
	activeGroups map[string]bool // groupID → statut actif
	// Nombre total de groupes actifs
	groupCount int
}

// NewAckMatrix crée une nouvelle matrice d'acquittement
func NewAckMatrix() *AckMatrix {
	return &AckMatrix{
		messages:     make(map[string]map[string]bool),
		activeGroups: make(map[string]bool),
	}
}

// RegisterGroup enregistre un nouveau groupe de consommateurs
func (m *AckMatrix) RegisterGroup(groupID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.activeGroups[groupID] = true
	m.groupCount = len(m.activeGroups)
}

// RemoveGroup supprime un groupe et retourne les IDs de messages complètement acquittés
func (m *AckMatrix) RemoveGroup(groupID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.activeGroups, groupID)
	m.groupCount = len(m.activeGroups)

	// Trouver les messages qui peuvent maintenant être supprimés
	messagesToDelete := []string{}
	for msgID, acks := range m.messages {
		// Marquer ce groupe comme acquitté (puisqu'il est parti)
		acks[groupID] = true

		// Vérifier si tous les groupes restants ont acquitté
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

	// Supprimer les messages entièrement acquittés de la matrice
	for _, msgID := range messagesToDelete {
		delete(m.messages, msgID)
	}

	return messagesToDelete
}

// Acknowledge marque un message comme acquitté par un groupe
// Retourne true si le message est maintenant entièrement acquitté
func (m *AckMatrix) Acknowledge(messageID, groupID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Vérifier que le groupe existe
	if !m.activeGroups[groupID] {
		return false
	}

	// Initialiser le suivi pour ce message si nécessaire
	if _, exists := m.messages[messageID]; !exists {
		m.messages[messageID] = make(map[string]bool, m.groupCount)
	}

	// Marquer comme acquitté
	m.messages[messageID][groupID] = true

	// Vérifier si tous les groupes ont acquitté
	allAcked := true
	for g := range m.activeGroups {
		if !m.messages[messageID][g] {
			allAcked = false
			break
		}
	}

	// Si entièrement acquitté, supprimer du suivi
	if allAcked {
		delete(m.messages, messageID)
	}

	return allAcked
}

// Exposer le nombre de groupes actifs
func (m *AckMatrix) GetActiveGroupCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.groupCount
}

// GetPendingMessageCount retourne le nombre de messages en attente pour un groupe
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

// GetPendingMessageCount retourne le nombre de messages en attente pour un groupe
func (m *AckMatrix) GetPendingMessageIDs(groupID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, m.GetPendingMessageCount(groupID))
	for id, acks := range m.messages {
		if !acks[groupID] {
			ids = append(ids, id)
		}
	}
	return ids
}
