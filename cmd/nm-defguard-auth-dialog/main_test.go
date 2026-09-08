package main

import (
	"strings"
	"testing"
)

func TestNoSecretResponses(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--service", serviceName}, "no-secret\ntrue\n\n\n"},
		{[]string{"--service", serviceName, "--external-ui-mode"}, "[VPN Plugin UI]\nVersion=2\n\n[no-secret]\nValue=true\nLabel=\nIsSecret=false\nShouldAsk=false\n"},
	} {
		var output, errors strings.Builder
		if code := run(test.args, strings.NewReader("DATA_KEY=location-id\nDATA_VAL=1\nDONE\n"), &output, &errors); code != 0 || output.String() != test.want {
			t.Fatalf("run() = %d, %q, %q", code, output.String(), errors.String())
		}
	}
}
