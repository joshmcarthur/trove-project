package modules

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
)

const (
	initialBackoff = time.Second
	maxBackoff     = 30 * time.Second
)

// RunSources supervises all source modules until ctx is cancelled. Each module
// runs in its own goroutine with restart and exponential backoff on exit.
func RunSources(ctx context.Context, j journal.Journal, blobsPath string, mods []Module) {
	var wg sync.WaitGroup
	for _, mod := range mods {
		if mod.Manifest.Kind != KindSource {
			continue
		}
		wg.Add(1)
		go func(mod Module) {
			defer wg.Done()
			superviseSource(ctx, j, blobsPath, mod)
		}(mod)
	}
	wg.Wait()
}

func superviseSource(ctx context.Context, j journal.Journal, blobsPath string, mod Module) {
	backoff := initialBackoff
	name := mod.Manifest.Name

	for {
		if ctx.Err() != nil {
			return
		}

		handle, err := StartSource(ctx, j, blobsPath, mod)
		if handle != nil {
			_ = handle.Close()
		}

		if ctx.Err() != nil {
			return
		}

		if err != nil {
			log.Printf("modules: source %q exited: %v; restarting in %s", name, err, backoff)
		} else {
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
