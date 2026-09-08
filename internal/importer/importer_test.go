package importer

import (
	"context"
	"os/exec"
	"reflect"
	"testing"

	"github.com/graipher/network-manager-defguard-plugin/internal/defguard"
)

type dgRunner struct{}

func (dgRunner) Run(context.Context, ...string) ([]byte, error) {
	return []byte(`{"locations":[{"id":7,"name":"Office North","instance":"Acme","endpoint":"vpn.example:51820"}]}`), nil
}

type commandRunner struct{ calls [][]string }

func (r *commandRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil, &exec.ExitError{}
}

func TestDryRunDoesNotMutate(t *testing.T) {
	r := &commandRunner{}
	if err := Run(context.Background(), true, false, dgRunner{}, r); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 1 || !reflect.DeepEqual(r.calls[0][:4], []string{"nmcli", "-g", "vpn.service-type", "connection"}) {
		t.Fatalf("calls: %#v", r.calls)
	}
}

func TestProfileName(t *testing.T) {
	location := defguard.Location{ID: 7, Instance: " Acme ", Name: "Office   North"}
	got := profileName(location, "all")
	if got != "Office North (Defguard)" {
		t.Fatalf("profileName()=%q", got)
	}
	if got := profileName(location, "predefined"); got != "Office North – predefined (Defguard)" {
		t.Fatalf("profileName(predefined)=%q", got)
	}
}

func TestProfileUUIDIsStableAndUserScoped(t *testing.T) {
	got := profileUUID("alice", 7, "all")
	if got != profileUUID("alice", 7, "all") || got == profileUUID("bob", 7, "all") || got == profileUUID("alice", 7, "predefined") {
		t.Fatalf("unstable or unscoped UUID: %q", got)
	}
	if len(got) != 36 || got[14] != '5' {
		t.Fatalf("invalid version-5 UUID: %q", got)
	}
}

func TestWithPredefinedDryRunChecksBothProfiles(t *testing.T) {
	r := &commandRunner{}
	if err := Run(context.Background(), true, true, dgRunner{}, r); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 2 {
		t.Fatalf("calls: %#v", r.calls)
	}
}

func TestDataValueEscapesNmcliDictionarySeparators(t *testing.T) {
	if got := dataValue(`one,two\three`); got != `one\,two\\three` {
		t.Fatalf("dataValue()=%q", got)
	}
}
