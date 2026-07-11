package modules

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/journal"
)

const (
	initialBackoff = time.Second
	maxBackoff     = 30 * time.Second
)

// RunModules supervises discovered modules until ctx is cancelled.
func RunModules(ctx context.Context, j journal.Journal, mods []Module, blobs blob.Store, httpRegistry *HTTPRegistry, eventRegistry *EventRegistry) {
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
			superviseModule(ctx, j, mod, blobs, httpRegistry, eventRegistry)
		}(mod)
	}
	wg.Wait()
}

func shouldSupervise(manifest Manifest) bool {
	return manifest.Kind == KindSource ||
		len(manifest.HTTPRoutes()) > 0 ||
		manifest.EventRoutes()
}

func superviseModule(ctx context.Context, j journal.Journal, mod Module, blobs blob.Store, httpRegistry *HTTPRegistry, eventRegistry *EventRegistry) {
	backoff := initialBackoff
	name := mod.Manifest.Name

	for {
		if ctx.Err() != nil {
			return
		}

		handle, err := StartSource(ctx, j, mod, blobs, httpRegistry, eventRegistry)
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

// RunSources supervises source and HTTP modules until ctx is cancelled.
func RunSources(ctx context.Context, j journal.Journal, mods []Module, blobs blob.Store, registry *HTTPRegistry) {
	RunModules(ctx, j, mods, blobs, registry, nil)
}

// RunProcessors supervises event-routing processors until ctx is cancelled.
func RunProcessors(ctx context.Context, j journal.Journal, mods []Module, blobs blob.Store, registry *EventRegistry) {
	RunModules(ctx, j, mods, blobs, nil, registry)
}

// RunSinks supervises event-routing sinks until ctx is cancelled.
func RunSinks(ctx context.Context, j journal.Journal, mods []Module, blobs blob.Store, registry *EventRegistry) {
	RunModules(ctx, j, mods, blobs, nil, registry)
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
