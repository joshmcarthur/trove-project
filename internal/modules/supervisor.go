package modules

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/types"
)

const (
	initialBackoff = time.Second
	maxBackoff     = 30 * time.Second
)

// RunModules supervises discovered modules until ctx is cancelled.
func RunModules(
	ctx context.Context,
	j journal.Journal,
	mods []Module,
	blobs blob.Store,
	httpRegistry *HTTPRegistry,
	mcpRegistry *MCPRegistry,
	eventRegistry *RevisionRegistry,
	mcpTools []MCPToolEntry,
	toolModules map[string]string,
	settings *SettingsStore,
	catalog *types.Catalog,
) {
	var wg sync.WaitGroup
	for _, mod := range mods {
		manifest, err := loadModuleManifest(mod)
		if err != nil {
			log.Printf("modules: load manifest %q: %v", mod.Manifest.Name, err)
			continue
		}
		if !shouldSupervise(manifest) {
			continue
		}
		wg.Add(1)
		go func(mod Module) {
			defer wg.Done()
			superviseModule(ctx, j, mod, blobs, httpRegistry, mcpRegistry, eventRegistry, mcpTools, toolModules, settings, catalog)
		}(mod)
	}
	wg.Wait()
}

func shouldSupervise(manifest Manifest) bool {
	return manifest.Kind == KindSource ||
		len(manifest.HTTPRoutes()) > 0 ||
		len(manifest.AuthValidators()) > 0 ||
		len(manifest.MCPTools()) > 0 ||
		manifest.EventRoutes()
}

func superviseModule(
	ctx context.Context,
	j journal.Journal,
	mod Module,
	blobs blob.Store,
	httpRegistry *HTTPRegistry,
	mcpRegistry *MCPRegistry,
	eventRegistry *RevisionRegistry,
	mcpTools []MCPToolEntry,
	toolModules map[string]string,
	settings *SettingsStore,
	catalog *types.Catalog,
) {
	backoff := initialBackoff
	name := mod.Manifest.Name

	for {
		if ctx.Err() != nil {
			return
		}

		handle, err := StartSource(ctx, j, mod, blobs, httpRegistry, mcpRegistry, eventRegistry, mcpTools, toolModules, settings, catalog)
		if handle != nil {
			select {
			case <-ctx.Done():
				_ = handle.Close()
				return
			case <-handle.done:
				_ = handle.Close()
			}
		} else if err != nil && ctx.Err() == nil {
			log.Printf("modules: module %q start failed: %v; restarting in %s", name, err, backoff)
		}

		if ctx.Err() != nil {
			return
		}

		if err == nil {
			log.Printf("modules: module %q exited; restarting in %s", name, backoff)
		}

		if !sleepOrDone(ctx, backoff) {
			return
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
