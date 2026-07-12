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
	"time"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/config"
	"github.com/joshmcarthur/trove/internal/gateway"
	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/modules"
	"github.com/joshmcarthur/trove/internal/modules/bundled"
	"github.com/joshmcarthur/trove/internal/types"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

// version is injected at build time via -ldflags; see version.go.
func main() {
	if name := strings.TrimSpace(os.Getenv(trovemodule.BundledModuleEnv)); name != "" {
		bundled.Serve(name)
		return
	}

	showVersion := flag.Bool("version", false, "print version and exit")
	configPath := flag.String("config", "", "path to trove.toml")
	flag.Parse()

	if *showVersion {
		fmt.Println(versionString())
		os.Exit(0)
	}

	args := flag.Args()
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "trove: -config is required")
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	mods, err := modules.Discover(cfg.Modules.Paths)
	if err != nil {
		log.Printf("trove: module discovery: %v", err)
	}

	if len(args) > 0 {
		if exitCode := tryRunCLICommand(cfg, mods, args); exitCode >= 0 {
			os.Exit(exitCode)
		}
	}

	runDaemon(cfg, mods)
}

func tryRunCLICommand(cfg config.Config, mods []modules.Module, args []string) int {
	cliCommands, err := modules.CollectCLICommands(mods)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	registry := modules.NewCLIRegistry(mods)
	mod, ok := registry.ModuleForCommand(cliCommands, args[0])
	if !ok {
		return -1
	}

	blobStore, err := blob.OpenFilesystem(cfg.Blobs.Path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	moduleTypes, err := modules.CollectModuleTypesInputs(mods)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	userTypes := make([]types.TypeDecl, len(cfg.Types))
	for i, td := range cfg.Types {
		userTypes[i] = types.TypeDecl{
			Name:    td.Name,
			Version: td.Version,
			Schema:  td.Schema,
		}
	}
	catalog, catalogWarnings, err := types.BuildCatalog(
		context.Background(),
		blobStore,
		types.DefaultBuiltinDir(),
		moduleTypes,
		userTypes,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, w := range catalogWarnings {
		log.Printf("trove: %s", w)
	}

	settingsStore, err := modules.NewSettingsStore(cfg.Modules)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer settingsStore.Close()

	stdout, stderr, exitCode, err := modules.RunCLICommand(
		context.Background(),
		mod,
		catalog,
		blobStore,
		args[0],
		args[1:],
		settingsStore,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(stdout) > 0 {
		os.Stdout.Write(stdout)
	}
	if len(stderr) > 0 {
		os.Stderr.Write(stderr)
	}
	return exitCode
}

func runDaemon(cfg config.Config, mods []modules.Module) {
	store, err := journal.Open(cfg.Journal.Path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()

	if cfg.Journal.RetentionDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -cfg.Journal.RetentionDays)
		if n, err := store.PruneBefore(context.Background(), cutoff); err != nil {
			log.Printf("trove: journal retention: %v", err)
		} else if n > 0 {
			log.Printf("trove: journal retention: pruned %d events older than %d days", n, cfg.Journal.RetentionDays)
		}
	}

	blobStore, err := blob.OpenFilesystem(cfg.Blobs.Path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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

	authValidatorRefs, err := modules.CollectAuthValidatorRefs(mods)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := modules.ValidateAuthConfig(cfg.HTTP.Auth.Validator, routes, authValidatorRefs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := gateway.ValidateRoutes(routes, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	httpRegistry := modules.NewHTTPRegistry()
	mcpRegistry := modules.NewMCPRegistry()
	eventRegistry := modules.NewEventRegistry()

	settingsStore, err := modules.NewSettingsStore(cfg.Modules)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer settingsStore.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	modules.WarnModuleCycles(mods)

	moduleTypes, err := modules.CollectModuleTypesInputs(mods)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	userTypes := make([]types.TypeDecl, len(cfg.Types))
	for i, td := range cfg.Types {
		userTypes[i] = types.TypeDecl{
			Name:    td.Name,
			Version: td.Version,
			Schema:  td.Schema,
		}
	}
	catalog, catalogWarnings, err := types.BuildCatalog(
		ctx,
		blobStore,
		types.DefaultBuiltinDir(),
		moduleTypes,
		userTypes,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, w := range catalogWarnings {
		log.Printf("trove: %s", w)
	}

	router := modules.NewRouter(store, eventRegistry)
	routerDone := make(chan struct{})
	go func() {
		defer close(routerDone)
		if err := router.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("trove: event router: %v", err)
		}
	}()

	gw, err := gateway.New(gateway.Config{
		Listen:        cfg.HTTP.Listen,
		MaxBodyBytes:  cfg.HTTP.MaxBodyBytes,
		AuthValidator: cfg.HTTP.Auth.Validator,
	}, routes, httpRegistry, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	gatewayDone := make(chan struct{})
	go func() {
		defer close(gatewayDone)
		if err := gw.Serve(ctx); err != nil && ctx.Err() == nil {
			log.Printf("trove: http gateway: %v", err)
		}
	}()

	modulesDone := make(chan struct{})
	go func() {
		modules.RunModules(ctx, store, mods, blobStore, httpRegistry, mcpRegistry, eventRegistry, mcpTools, toolModules, settingsStore, catalog)
		close(modulesDone)
	}()

	<-ctx.Done()
	log.Printf("trove: shutting down")

	<-gatewayDone
	<-routerDone
	<-modulesDone
}

func supervisedModuleNames(mods []modules.Module) []string {
	names := make([]string, 0, len(mods))
	for _, mod := range mods {
		manifest := mod.Manifest
		if manifest.Kind == modules.KindSource ||
			len(manifest.HTTPRoutes()) > 0 ||
			len(manifest.AuthValidators()) > 0 ||
			len(manifest.MCPTools()) > 0 ||
			len(manifest.CLICommands()) > 0 ||
			manifest.EventRoutes() {
			names = append(names, manifest.Name)
		}
	}
	return names
}
