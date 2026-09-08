package main

import (
	"context"
	"flag"
	"log"

	"github.com/graipher/network-manager-defguard-plugin/internal/defguard"
	"github.com/graipher/network-manager-defguard-plugin/internal/importer"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "show profiles without changing NetworkManager")
	withPredefined := flag.Bool("with-predefined", false, "also import a predefined-traffic profile for each location")
	flag.Parse()
	if err := importer.Run(context.Background(), *dryRun, *withPredefined, defguard.CommandRunner{}, nil); err != nil {
		log.Fatal(err)
	}
}
