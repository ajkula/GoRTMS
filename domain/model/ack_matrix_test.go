package model

import (
	"testing"
)

func TestAckMatrix(t *testing.T) {
	// Créer une nouvelle matrice
	matrix := NewAckMatrix()

	// Enregistrer des groupes
	matrix.RegisterGroup("group1")
	matrix.RegisterGroup("group2")

	// Vérifier le nombre de groupes
	if count := matrix.GetActiveGroupCount(); count != 2 {
		t.Errorf("Expected 2 active groups, got %d", count)
	}

	// Tester l'acquittement
	if acked := matrix.Acknowledge("msg1", "group1"); acked {
		t.Error("Expected msg1 to not be fully acknowledged yet")
	}

	if acked := matrix.Acknowledge("msg1", "group2"); !acked {
		t.Error("Expected msg1 to be fully acknowledged")
	}

	// Tester la suppression de groupe
	messagesToDelete := matrix.RemoveGroup("group1")
	if len(messagesToDelete) != 0 {
		t.Errorf("Expected 0 messages to delete, got %d", len(messagesToDelete))
	}

	// Ajouter un nouveau message
	matrix.Acknowledge("msg2", "group2")

	// Le groupe restant a déjà acquitté, donc le message doit être supprimé
	if !matrix.Acknowledge("msg2", "group2") {
		t.Error("Expected msg2 to be fully acknowledged")
	}
}
