package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
)

const serviceName = "org.freedesktop.NetworkManager.defguard"

func run(args []string, input io.Reader, output, errors io.Writer) int {
	flags := flag.NewFlagSet("nm-defguard-auth-dialog", flag.ContinueOnError)
	flags.SetOutput(errors)
	var service, ignored string
	var external, ignoredBool bool
	flags.StringVar(&service, "service", "", "VPN service type")
	flags.StringVar(&service, "s", "", "VPN service type")
	for _, name := range []string{"uuid", "u", "name", "n", "hint", "t"} {
		flags.StringVar(&ignored, name, "", "")
	}
	flags.BoolVar(&external, "external-ui-mode", false, "external UI mode")
	for _, name := range []string{"allow-interaction", "i", "reprompt", "r"} {
		flags.BoolVar(&ignoredBool, name, false, "")
	}
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errors, "unexpected positional arguments")
		return 1
	}
	if service != serviceName {
		fmt.Fprintf(errors, "unsupported VPN service %q\n", service)
		return 1
	}

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		if scanner.Text() == "DONE" {
			if external {
				fmt.Fprint(output, "[VPN Plugin UI]\nVersion=2\n\n[no-secret]\nValue=true\nLabel=\nIsSecret=false\nShouldAsk=false\n")
			} else {
				fmt.Fprint(output, "no-secret\ntrue\n\n\n")
			}
			return 0
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(errors, "read VPN details: %v\n", err)
	} else {
		fmt.Fprintln(errors, "read VPN details: missing DONE marker")
	}
	return 1
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
