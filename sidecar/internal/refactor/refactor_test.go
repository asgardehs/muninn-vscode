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

func TestValidateNames_RejectsEmpty(t *testing.T) {
	if err := ValidateNames("", "x"); err == nil {
		t.Error("expected error for empty old name")
	}
	if err := ValidateNames("x", ""); err == nil {
		t.Error("expected error for empty new name")
	}
}

func TestValidateNames_RejectsSame(t *testing.T) {
	if err := ValidateNames("foo.bar", "foo.bar"); err == nil {
		t.Error("expected error for identical names (rename cycle)")
	}
}

func TestValidateNames_RejectsBadCharacters(t *testing.T) {
	if err := ValidateNames("foo.bar", "foo/bar"); err == nil {
		t.Error("expected error for slash in name")
	}
	if err := ValidateNames("foo.bar", "foo bar"); err == nil {
		t.Error("expected error for space in name")
	}
	if err := ValidateNames("foo.bar", ".foo"); err == nil {
		t.Error("expected error for leading dot")
	}
	if err := ValidateNames("foo.bar", "foo."); err == nil {
		t.Error("expected error for trailing dot")
	}
}

func TestValidateNames_AllowsNormal(t *testing.T) {
	if err := ValidateNames("people.john-smith", "people.john-doe"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
