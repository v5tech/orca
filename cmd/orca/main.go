package main

import (
	"fmt"
	"io"
	"os"

	"github.com/v5tech/orca/internal/cli"
)

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if err := cli.Run(args, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "orca:", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
