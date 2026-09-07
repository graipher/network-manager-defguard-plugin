package defguard

import "testing"

func TestNewInterface(t *testing.T) {
	before := Status{Active: []Active{{Type: "location", Name: "office", Interface: "wg0"}}}
	after := Status{Active: append(before.Active, Active{Type: "location", Name: "office", Interface: "wg1"})}
	got, err := NewInterface(before, after, "office")
	if err != nil || got != "wg1" {
		t.Fatalf("NewInterface() = %q, %v", got, err)
	}
}

func TestNewInterfaceRejectsAmbiguousExistingConnection(t *testing.T) {
	status := Status{Active: []Active{
		{Type: "location", Name: "office", Interface: "wg0"},
		{Type: "location", Name: "office", Interface: "wg1"},
	}}
	if _, err := NewInterface(status, status, "office"); err == nil {
		t.Fatal("expected ambiguous location error")
	}
}

func TestHasInterfaceCanIgnoreDisplayName(t *testing.T) {
	status := Status{Active: []Active{{Type: "location", Name: "office", Interface: "wg0"}}}
	if !HasInterface(status, "wg0", "") {
		t.Fatal("expected interface match")
	}
}
