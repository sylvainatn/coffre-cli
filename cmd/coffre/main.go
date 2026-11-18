package main

import (
	"os"

	"github.com/coffre-dev/coffre-cli/internal/cli"
)

// Injectées à la compilation par GoReleaser (-ldflags "-X main.version=...")
var (
	version       = "dev"
	defaultAPIURL = "http://localhost:4000"
)

func main() {
	os.Exit(cli.Execute(version, defaultAPIURL))
}
