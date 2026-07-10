package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/joshmcarthur/trove/internal/config"
	"github.com/joshmcarthur/trove/internal/journal"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	configPath := flag.String("config", "", "path to trove.toml")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "trove: -config is required")
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	store, err := journal.Open(cfg.Journal.Path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()

	fmt.Fprintln(os.Stderr, "trove: journal ready; module runtime not yet implemented")
	os.Exit(1)
}
