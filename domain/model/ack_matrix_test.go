package model

import (
	"testing"
)

func TestAckMatrix(t *testing.T) {
	// Create a new matrix
	matrix := NewAckMatrix()

	// Register some groups
	matrix.RegisterGroup("group1")
	matrix.RegisterGroup("group2")

	// Check the number of active groups
	if count := matrix.GetActiveGroupCount(); count != 2 {
		t.Errorf("Expected 2 active groups, got %d", count)
	}

	// Test message acknowledgment
	if acked := matrix.Acknowledge("msg1", "group1"); acked {
		t.Error("Expected msg1 to not be fully acknowledged yet")
	}

	if acked := matrix.Acknowledge("msg1", "group2"); !acked {
		t.Error("Expected msg1 to be fully acknowledged")
	}

	// Test group removal
	messagesToDelete := matrix.RemoveGroup("group1")
	if len(messagesToDelete) != 0 {
		t.Errorf("Expected 0 messages to delete, got %d", len(messagesToDelete))
	}

	// Add a new message
	matrix.Acknowledge("msg2", "group2")

	// The remaining group has already acknowledged, so the message should be removed
	if !matrix.Acknowledge("msg2", "group2") {
		t.Error("Expected msg2 to be fully acknowledged")
	}
}

func TestAckMatrix_BasicFlow(t *testing.T) {
	matrix := NewAckMatrix()

	// Register two groups
	matrix.RegisterGroup("group1")
	matrix.RegisterGroup("group2")

	if got := matrix.GetActiveGroupCount(); got != 2 {
		t.Errorf("Expected 2 active groups, got %d", got)
	}

	// Acknowledge from only one group
	if acked := matrix.Acknowledge("msg1", "group1"); acked {
		t.Error("Expected msg1 to not be fully acknowledged yet")
	}

	// Acknowledge from the second group
	if acked := matrix.Acknowledge("msg1", "group2"); !acked {
		t.Error("Expected msg1 to be fully acknowledged")
	}

	// Try to acknowledge a message from an unknown group
	if acked := matrix.Acknowledge("msg1", "unknown-group"); acked {
		t.Error("Expected acknowledgment from unknown group to fail")
	}

	// Remove one group
	deleted := matrix.RemoveGroup("group1")
	if len(deleted) != 0 {
		t.Errorf("Expected 0 messages to delete, got %d", len(deleted))
	}

	// Add a message already acknowledged by remaining group
	if !matrix.Acknowledge("msg2", "group2") {
		t.Error("Expected msg2 to be fully acknowledged immediately")
	}
}

func TestAckMatrix_EdgeCases(t *testing.T) {
	matrix := NewAckMatrix()

	// No groups yet
	if acked := matrix.Acknowledge("msg0", "nonexistent"); acked {
		t.Error("Expected false for acknowledgment from nonexistent group")
	}

	// Register a group, then acknowledge
	matrix.RegisterGroup("groupA")
	if acked := matrix.Acknowledge("msg1", "groupA"); !acked {
		t.Error("Expected msg1 to be immediately acknowledged with one group")
	}

	// Register more groups after acknowledgment
	matrix.RegisterGroup("groupB")
	matrix.RegisterGroup("groupC")

	// Acknowledge msg2 only partially
	matrix.Acknowledge("msg2", "groupA")
	matrix.Acknowledge("msg2", "groupB")

	// Still pending, groupC hasn't acked yet
	if matrix.Acknowledge("msg2", "groupC") != true {
		t.Error("Expected msg2 to be fully acknowledged after last group")
	}

	// [CHECK] Acknowledge a message twice
	matrix.RegisterGroup("groupD")
	matrix.Acknowledge("msg3", "groupD")
	if matrix.Acknowledge("msg3", "groupD") {
		t.Error("Acknowledging twice should neither fail nor alter state")
	}
}

func TestAckMatrix_GetPendingMessageIDs(t *testing.T) {
	matrix := NewAckMatrix()

	matrix.RegisterGroup("g1")
	matrix.RegisterGroup("g2")

	// msgX is pending for both
	matrix.Acknowledge("msgX", "g1")

	ids := matrix.GetPendingMessageIDs("g2")
	if len(ids) == 0 || ids[0] != "msgX" {
		t.Errorf("Expected msgX to be pending for g2, got %v", ids)
	}
}

func TestAckMatrix_RemoveGroupCompletesMessages(t *testing.T) {
	matrix := NewAckMatrix()

	matrix.RegisterGroup("g1")
	matrix.RegisterGroup("g2")
	matrix.Acknowledge("m1", "g1")
	matrix.Acknowledge("m1", "g2")

	// Already acknowledged by both → should be auto-removed
	if matrix.Acknowledge("m1", "g2") != false {
		t.Error("Message should already be removed after full acknowledgment")
	}

	// Add another message only acknowledged by one group
	matrix.Acknowledge("m2", "g1")

	// Remove g2 → should mark m2 as fully acked
	deleted := matrix.RemoveGroup("g2")
	if len(deleted) != 1 || deleted[0] != "m2" {
		t.Errorf("Expected m2 to be deleted on group removal, got %v", deleted)
	}
}
