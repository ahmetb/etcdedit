package main

import (
	"os"

	"github.com/abalkan/etcdedit/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
