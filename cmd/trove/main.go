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

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/config"
	"github.com/joshmcarthur/trove/internal/gateway"
	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/modules"
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

	blobStore, err := blob.OpenFilesystem(cfg.Blobs.Path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	mods, err := modules.Discover(cfg.Modules.Paths)
	if err != nil {
		log.Printf("trove: module discovery: %v", err)
	}

	moduleNames := supervisedModuleNames(mods)
	if len(moduleNames) > 0 {
		log.Printf("trove: starting modules: %s", strings.Join(moduleNames, ", "))
	} else {
		log.Printf("trove: no modules discovered")
	}

	routes, err := modules.CollectHTTPRoutes(mods)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	mcpTools, err := modules.CollectMCPTools(mods)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	toolModules := modules.MCPToolModuleIndex(mcpTools)

	if err := gateway.ValidateRoutes(routes, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	httpRegistry := modules.NewHTTPRegistry()
	mcpRegistry := modules.NewMCPRegistry()
	eventRegistry := modules.NewEventRegistry()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	modules.WarnModuleCycles(mods)

	go modules.RunModules(ctx, store, mods, blobStore, httpRegistry, mcpRegistry, eventRegistry, mcpTools, toolModules)

	router := modules.NewRouter(store, eventRegistry)
	go func() {
		if err := router.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("trove: event router: %v", err)
		}
	}()

	gw, err := gateway.New(gateway.Config{
		Listen:       cfg.HTTP.Listen,
		MaxBodyBytes: cfg.HTTP.MaxBodyBytes,
	}, routes, httpRegistry, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	go func() {
		if err := gw.Serve(ctx); err != nil && ctx.Err() == nil {
			log.Printf("trove: http gateway: %v", err)
		}
	}()

	<-ctx.Done()
}

func supervisedModuleNames(mods []modules.Module) []string {
	names := make([]string, 0, len(mods))
	for _, mod := range mods {
		manifest := mod.Manifest
		if manifest.Kind == modules.KindSource ||
			len(manifest.HTTPRoutes()) > 0 ||
			len(manifest.MCPTools()) > 0 ||
			manifest.EventRoutes() {
			names = append(names, manifest.Name)
		}
	}
	return names
}
