package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joshmcarthur/trove/internal/config"
	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/modules"
	"github.com/joshmcarthur/trove/internal/query"
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

	mods, err := modules.Discover(cfg.Modules.Paths)
	if err != nil {
		log.Printf("trove: module discovery: %v", err)
	}

	sourceNames := make([]string, 0, len(mods))
	for _, mod := range mods {
		if mod.Manifest.Kind == modules.KindSource {
			sourceNames = append(sourceNames, mod.Manifest.Name)
		}
	}
	if len(sourceNames) > 0 {
		log.Printf("trove: starting source modules: %s", strings.Join(sourceNames, ", "))
	} else {
		log.Printf("trove: no source modules discovered")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	querySvc := &query.Service{Journal: store}
	go func() {
		if err := query.Serve(ctx, cfg.MCP.Listen, querySvc); err != nil && ctx.Err() == nil {
			log.Printf("trove: mcp server: %v", err)
		}
	}()

	go modules.RunSources(ctx, store, mods)
	<-ctx.Done()
}
