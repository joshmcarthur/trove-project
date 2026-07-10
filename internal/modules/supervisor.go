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

// RunSources supervises source and HTTP modules until ctx is cancelled.
func RunSources(ctx context.Context, j journal.Journal, mods []Module, blobs blob.Store, registry *HTTPRegistry) {
	var wg sync.WaitGroup
	for _, mod := range mods {
		manifest, err := loadModuleManifest(mod)
		if err != nil {
			log.Printf("modules: load manifest %q: %v", mod.Manifest.Name, err)
			continue
		}
		if manifest.Kind != KindSource && len(manifest.HTTPRoutes()) == 0 {
			continue
		}
		wg.Add(1)
		go func(mod Module) {
			defer wg.Done()
			superviseSource(ctx, j, mod, blobs, registry)
		}(mod)
	}
	wg.Wait()
}

func superviseSource(ctx context.Context, j journal.Journal, mod Module, blobs blob.Store, registry *HTTPRegistry) {
	backoff := initialBackoff
	name := mod.Manifest.Name

	for {
		if ctx.Err() != nil {
			return
		}

		handle, err := StartSource(ctx, j, mod, blobs, registry)
		if handle != nil {
			select {
			case <-ctx.Done():
				_ = handle.Close()
				return
			case <-handle.done:
				_ = handle.Close()
			}
		} else if err != nil && ctx.Err() == nil {
			log.Printf("modules: source %q start failed: %v; restarting in %s", name, err, backoff)
		}

		if ctx.Err() != nil {
			return
		}

		if err == nil {
			log.Printf("modules: source %q exited; restarting in %s", name, backoff)
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
