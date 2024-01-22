package main

import (
	"github.com/thomiceli/opengist/internal/cli"
	"os"
)

func main() {
	if err := cli.App(); err != nil {
		os.Exit(1)
	}
}
