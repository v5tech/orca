package main

import (
	"fmt"
	"os"

	"github.com/v5tech/orca/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "orca:", err)
		os.Exit(1)
	}
}
