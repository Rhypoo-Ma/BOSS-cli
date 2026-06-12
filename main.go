package main

import (
	"os"

	"github.com/Rhypoo-Ma/BOSS-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
