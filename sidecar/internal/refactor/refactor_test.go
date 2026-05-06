package refactor

import "testing"

func TestPlan_ZeroValueIsEmpty(t *testing.T) {
	var p Plan
	if len(p.FileEdits) != 0 {
		t.Errorf("FileEdits should be empty, got %d", len(p.FileEdits))
	}
	if p.RenameFrom != "" || p.RenameTo != "" {
		t.Errorf("Rename fields should be empty")
	}
}
