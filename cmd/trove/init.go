package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/joshmcarthur/trove/internal/config"
)

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", ".", "directory to initialize")
	force := fs.Bool("force", false, "overwrite an existing trove.toml")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "trove init: unexpected argument %q\n", fs.Arg(0))
		return 2
	}

	result, err := config.Scaffold(*dir, *force)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Printf("Created %s\n", result.ConfigPath)
	fmt.Printf("Created %s/\n", result.BlobsPath)
	fmt.Printf("\nStart Trove with:\n\n  trove -config %s\n", result.ConfigPath)
	return 0
}
