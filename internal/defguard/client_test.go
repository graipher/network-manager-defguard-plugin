package defguard

import (
	"context"
	"reflect"
	"testing"
)

type recordingRunner struct{ args []string }

func (r *recordingRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	r.args = args
	return nil, nil
}

func TestConnectTrafficModes(t *testing.T) {
	for mode, flag := range map[string]string{"all": "--all-traffic", "predefined": "--predefined-traffic"} {
		r := &recordingRunner{}
		if err := (Client{Runner: r}).Connect(context.Background(), 7, "Acme", mode); err != nil {
			t.Fatal(err)
		}
		want := []string{"connect", "--id", "7", "--json", "--instance", "Acme", flag}
		if !reflect.DeepEqual(r.args, want) {
			t.Fatalf("Connect(%q) args = %#v, want %#v", mode, r.args, want)
		}
	}
}

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
