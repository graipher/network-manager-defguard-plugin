package nmplugin

import (
	"encoding/xml"
	"os/user"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestParseActivation(t *testing.T) {
	settings := ConnectionSettings{
		"connection": {"permissions": dbus.MakeVariant([]string{"user:" + currentUser(t) + ":"})},
		"vpn":        {"data": dbus.MakeVariant(map[string]string{"location-id": "7", "instance": "acme", "user": currentUser(t), "endpoint": "127.0.0.1:51820"}), "secrets": dbus.MakeVariant(map[string]string{"defguard-connected": "true", "defguard-interface": "wg7"})},
	}
	a, err := parseActivation(settings)
	if err != nil || a.id != 7 || a.user != currentUser(t) {
		t.Fatalf("parseActivation() = %#v, %v", a, err)
	}
}

func TestIntrospectionXMLIsWellFormed(t *testing.T) {
	var value any
	if err := xml.Unmarshal([]byte(IntrospectionXML), &value); err != nil {
		t.Fatal(err)
	}
}

func TestParseActivationRejectsMismatchedUser(t *testing.T) {
	settings := ConnectionSettings{
		"connection": {"permissions": dbus.MakeVariant([]string{"user:someone-else:"})},
		"vpn":        {"data": dbus.MakeVariant(map[string]string{"location-id": "7", "user": currentUser(t)}), "secrets": dbus.MakeVariant(map[string]string{"defguard-connected": "true", "defguard-interface": "wg7"})},
	}
	if _, err := parseActivation(settings); err == nil {
		t.Fatal("expected permission mismatch")
	}
}

func currentUser(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	return u.Username
}
