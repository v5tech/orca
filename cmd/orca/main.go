package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/v5tech/orca/internal/cli"
)

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	err := cli.Run(args, stdout, stderr)
	if err == nil {
		return 0
	}
	if errors.Is(err, cli.ErrChangesNeeded) {
		return 1
	}
	fmt.Fprintln(stderr, "orca:", err)
	return 2
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
