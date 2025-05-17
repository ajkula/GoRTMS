package service

import (
	"context"
	"errors"
	"log"
	"time"

	"slices"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

var (
	ErrConsumerGroupNotFound = errors.New("consumer group not found")
	ErrInvalidTTL            = errors.New("invalid TTL")
)

// ConsumerGroupServiceImpl implémente le service des consumer groups
type ConsumerGroupServiceImpl struct {
	consumerGroupRepo outbound.ConsumerGroupRepository
	messageRepo       outbound.MessageRepository
	rootCtx           context.Context
}

// NewConsumerGroupService crée un nouveau service de consumer groups
func NewConsumerGroupService(
	consumerGroupRepo outbound.ConsumerGroupRepository,
	messageRepo outbound.MessageRepository,
	rootCtx context.Context,
) inbound.ConsumerGroupService {
	service := &ConsumerGroupServiceImpl{
		consumerGroupRepo: consumerGroupRepo,
		messageRepo:       messageRepo,
		rootCtx:           rootCtx,
	}

	// Démarrer la tâche de nettoyage périodique
	service.startCleanupTask(rootCtx)

	return service
}

// ListConsumerGroups liste tous les consumer groups d'une queue
func (s *ConsumerGroupServiceImpl) ListConsumerGroups(
	ctx context.Context,
	domainName, queueName string,
) ([]*model.ConsumerGroup, error) {
	// Récupérer la liste des IDs de groupes
	groupIDs, err := s.consumerGroupRepo.ListGroups(ctx, domainName, queueName)
	if err != nil {
		return nil, err
	}

	// Convertir en objets ConsumerGroup complets
	groups := make([]*model.ConsumerGroup, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		group, err := s.getGroupDetails(ctx, domainName, queueName, groupID)
		if err != nil {
			log.Printf("Error getting details for group %s: %v", groupID, err)
			continue
		}
		groups = append(groups, group)
	}

	return groups, nil
}

// ListAllGroups liste tous les consumer groups de tous les domaines
func (s *ConsumerGroupServiceImpl) ListAllGroups(ctx context.Context) ([]*model.ConsumerGroup, error) {
	// Cette méthode devrait parcourir tous les domaines et toutes les queues
	// pour récupérer tous les consumer groups. L'implémentation exacte dépend
	// de l'implémentation du repository.

	// Si le repository a une méthode GetAllGroups, on peut l'utiliser directement
	if repo, ok := s.consumerGroupRepo.(interface {
		GetAllGroups(ctx context.Context) ([]*model.ConsumerGroup, error)
	}); ok {
		return repo.GetAllGroups(ctx)
	}

	// Sinon, on implémente une version basique qui parcourt les domaines et queues
	// Note: Cette implémentation est inefficace mais fonctionnelle. Idéalement,
	// ajoutez une méthode GetAllGroups au repository.

	// On suppose qu'on a accès au DomainRepository et QueueRepository
	// (ces références devraient être ajoutées au service)

	// Exemple simplifié (à adapter selon votre architecture réelle):
	allGroups := []*model.ConsumerGroup{}

	// Remplacer par la logique pour parcourir les domaines et les queues
	// puis appeler ListConsumerGroups pour chaque paire domaine/queue

	// Pour l'instant, retourne juste une liste vide
	// Cette méthode doit être complétée avec l'accès aux repositories
	return allGroups, nil
}

// GetConsumerGroup récupère les détails d'un consumer group
func (s *ConsumerGroupServiceImpl) GetConsumerGroup(
	ctx context.Context,
	domainName, queueName, groupID string,
) (*model.ConsumerGroup, error) {
	return s.getGroupDetails(ctx, domainName, queueName, groupID)
}

// CreateConsumerGroup crée un nouveau consumer group
func (s *ConsumerGroupServiceImpl) CreateConsumerGroup(
	ctx context.Context,
	domainName, queueName, groupID string,
	ttl time.Duration,
) error {
	// Enregistrer le consumer group
	if err := s.consumerGroupRepo.RegisterConsumer(ctx, domainName, queueName, groupID, ""); err != nil {
		return err
	}

	// Si un TTL est spécifié, l'enregistrer
	if ttl > 0 {
		// On suppose que vous avez ajouté cette méthode au repository
		if storeTTLRepo, ok := s.consumerGroupRepo.(interface {
			StoreTTL(ctx context.Context, domainName, queueName, groupID string, ttl time.Duration) error
		}); ok {
			if err := storeTTLRepo.StoreTTL(ctx, domainName, queueName, groupID, ttl); err != nil {
				return err
			}
		} else {
			// Fallback si l'interface n'est pas implémentée
			log.Printf("WARNING: StoreTTL not implemented in repository, TTL will not be stored")
		}
	}

	return nil
}

// DeleteConsumerGroup supprime un consumer group
func (s *ConsumerGroupServiceImpl) DeleteConsumerGroup(
	ctx context.Context,
	domainName, queueName, groupID string,
) error {
	// Récupérer la liste des consumers du groupe
	group, err := s.getGroupDetails(ctx, domainName, queueName, groupID)
	if err != nil {
		return err
	}

	// Supprimer chaque consumer
	for _, consumerID := range group.ConsumerIDs {
		if err := s.consumerGroupRepo.RemoveConsumer(ctx, domainName, queueName, groupID, consumerID); err != nil {
			return err
		}
	}

	// Nettoyer la matrice d'acquittement
	matrix := s.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
	messageIDs := matrix.RemoveGroup(groupID)

	// Supprimer les messages entièrement acquittés
	for _, messageID := range messageIDs {
		if err := s.messageRepo.DeleteMessage(ctx, domainName, queueName, messageID); err != nil {
			log.Printf("Error deleting message %s: %v", messageID, err)
		}
	}

	return nil
}

// UpdateConsumerGroupTTL met à jour le TTL d'un consumer group
func (s *ConsumerGroupServiceImpl) UpdateConsumerGroupTTL(
	ctx context.Context,
	domainName, queueName, groupID string,
	ttl time.Duration,
) error {
	// Vérifier que le groupe existe
	_, err := s.getGroupDetails(ctx, domainName, queueName, groupID)
	if err != nil {
		return err
	}

	// Mettre à jour le TTL dans le repository
	if storeTTLRepo, ok := s.consumerGroupRepo.(interface {
		StoreTTL(ctx context.Context, domainName, queueName, groupID string, ttl time.Duration) error
	}); ok {
		return storeTTLRepo.StoreTTL(ctx, domainName, queueName, groupID, ttl)
	}

	// Si l'interface n'est pas implémentée
	return errors.New("TTL update not supported by repository")
}

// CleanupStaleGroups nettoie les groupes inactifs
func (s *ConsumerGroupServiceImpl) CleanupStaleGroups(
	ctx context.Context,
	olderThan time.Duration,
) error {
	// Utiliser la méthode du repository pour le nettoyage
	return s.consumerGroupRepo.CleanupStaleGroups(ctx, olderThan)
}

// getGroupDetails récupère les détails d'un groupe spécifique
func (s *ConsumerGroupServiceImpl) getGroupDetails(
	ctx context.Context,
	domainName, queueName, groupID string,
) (*model.ConsumerGroup, error) {
	// Si le repository a une méthode GetGroupDetails, l'utiliser
	if repo, ok := s.consumerGroupRepo.(interface {
		GetGroupDetails(ctx context.Context, domainName, queueName, groupID string) (*model.ConsumerGroup, error)
	}); ok {
		return repo.GetGroupDetails(ctx, domainName, queueName, groupID)
	}

	// Sinon, construire manuellement à partir des informations disponibles
	position, err := s.consumerGroupRepo.GetPosition(ctx, domainName, queueName, groupID)
	if err != nil {
		// Vérifier si le groupe existe en récupérant la liste des groupes
		groups, err := s.consumerGroupRepo.ListGroups(ctx, domainName, queueName)
		if err != nil {
			return nil, err
		}

		groupExists := slices.Contains(groups, groupID)

		if !groupExists {
			return nil, ErrConsumerGroupNotFound
		}
	}

	// Récupérer les IDs de consommateurs
	consumerIDs := []string{}
	if repo, ok := s.consumerGroupRepo.(interface {
		GetConsumerIDs(ctx context.Context, domainName, queueName, groupID string) ([]string, error)
	}); ok {
		ids, err := repo.GetConsumerIDs(ctx, domainName, queueName, groupID)
		if err != nil {
			log.Printf("Warning: Could not retrieve consumer IDs: %v", err)
		} else {
			consumerIDs = ids
		}
	}

	// Récupérer le TTL
	var ttl time.Duration
	if repo, ok := s.consumerGroupRepo.(interface {
		GetTTL(ctx context.Context, domainName, queueName, groupID string) (time.Duration, error)
	}); ok {
		t, err := repo.GetTTL(ctx, domainName, queueName, groupID)
		if err != nil {
			log.Printf("Warning: Could not retrieve TTL: %v", err)
		} else {
			ttl = t
		}
	}

	// Récupérer les timestamps
	var createdAt, lastActivity time.Time
	if repo, ok := s.consumerGroupRepo.(interface {
		GetCreationTime(ctx context.Context, domainName, queueName, groupID string) (time.Time, error)
		GetLastActivity(ctx context.Context, domainName, queueName, groupID string) (time.Time, error)
	}); ok {
		createdAt, _ = repo.GetCreationTime(ctx, domainName, queueName, groupID)
		lastActivity, _ = repo.GetLastActivity(ctx, domainName, queueName, groupID)
	}

	// Compter les messages en attente
	messageCount := 0
	matrix := s.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
	if matrix != nil {
		messageCount = matrix.GetPendingMessageCount(groupID)
	}

	// Construire un objet ConsumerGroup basique
	// Note: Les champs ConsumerIDs, TTL, CreatedAt, LastActivity, MessageCount
	// ne sont pas disponibles avec cette approche basique
	group := &model.ConsumerGroup{
		DomainName:   domainName,
		QueueName:    queueName,
		GroupID:      groupID,
		Position:     position,
		ConsumerIDs:  consumerIDs,  // Vide si non disponible
		TTL:          ttl,          // Zéro si non disponible
		CreatedAt:    createdAt,    // Valeur zéro si non disponible
		LastActivity: lastActivity, // Valeur zéro si non disponible
		MessageCount: messageCount, // Zéro si non disponible
	}

	return group, nil
}

// startCleanupTask démarre une tâche périodique pour nettoyer les groupes inactifs
func (s *ConsumerGroupServiceImpl) startCleanupTask(ctx context.Context) {
	go func() {
		// Nettoyer toutes les 5 minutes
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Utiliser une durée raisonnable pour définir l'inactivité (1 heure)
				// ATTENTION: olderThan était 0, ce qui supprimait immédiatement TOUS les groupes!
				if err := s.CleanupStaleGroups(ctx, 1*time.Hour); err != nil {
					log.Printf("Error cleaning up stale consumer groups: %v", err)
				}
			}
		}
	}()
}
