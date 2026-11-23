// Command update-allowlist generates DaoCloud registry data for internal/imageref.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/v5tech/orca/internal/allowlist"
)

const defaultRef = "a195786892746d03f11d3ef1ed360b703e7ea91b"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "update-allowlist:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("update-allowlist", flag.ContinueOnError)
	ref := flags.String("ref", defaultRef, "DaoCloud public-image-mirror git ref")
	output := flags.String("output", "internal/imageref/allowlist_generated.go", "generated Go file path")
	if err := flags.Parse(args); err != nil {
		return err
	}

	data, err := allowlist.Fetch(*ref)
	if err != nil {
		return err
	}

	code := allowlist.RenderGo(data)
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		return err
	}
	temporary := *output + ".tmp"
	if err := os.WriteFile(temporary, []byte(code), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, *output)
}
