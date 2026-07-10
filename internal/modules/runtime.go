package modules

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

// healthcheckInterval is the period between module healthchecks.
var healthcheckInterval = 30 * time.Second

// SourceHandle supervises a running source module subprocess.
type SourceHandle struct {
	client *plugin.Client
	cancel context.CancelFunc
	done   chan struct{}
}

// Close stops the supervised module subprocess.
func (h *SourceHandle) Close() error {
	if h == nil {
		return nil
	}
	if h.cancel != nil {
		h.cancel()
	}
	if h.done != nil {
		<-h.done
	}
	if h.client != nil {
		h.client.Kill()
	}
	return nil
}

// StartSource launches a supervised module subprocess (sources and HTTP modules).
func StartSource(ctx context.Context, j journal.Journal, mod Module, blobs blob.Store, registry *HTTPRegistry) (*SourceHandle, error) {
	manifest, err := loadModuleManifest(mod)
	if err != nil {
		return nil, fmt.Errorf("modules: start %q: %w", mod.Manifest.Name, err)
	}

	hasHTTP := len(manifest.HTTPRoutes()) > 0
	needsIngest := manifest.Kind == KindSource

	switch {
	case needsIngest:
	case hasHTTP:
	default:
		return nil, fmt.Errorf("modules: start %q: not a source and no http routes", mod.Manifest.Name)
	}

	policy, err := LoadIngestPolicy(manifest, mod.Dir)
	if err != nil {
		return nil, fmt.Errorf("modules: start %q: %w", mod.Manifest.Name, err)
	}

	cmd, err := moduleExecCmd(mod)
	if err != nil {
		return nil, fmt.Errorf("modules: start %q: %w", mod.Manifest.Name, err)
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  trovemodule.Handshake,
		Plugins:          hostPluginSet(j, policy, manifest.Name, blobs, hasHTTP, needsIngest),
		Cmd:              cmd,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           hclog.NewNullLogger(),
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("modules: start source %q: client: %w", mod.Manifest.Name, err)
	}

	raw, err := rpcClient.Dispense(trovemodule.PluginName)
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("modules: start source %q: dispense: %w", mod.Manifest.Name, err)
	}

	sourceModule, ok := raw.(*sourceModuleClient)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("modules: start source %q: unexpected plugin type %T", mod.Manifest.Name, raw)
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	done := make(chan struct{})

	if hasHTTP && registry != nil {
		registry.Register(manifest.Name, sourceModule)
	}

	go func() {
		defer close(done)
		if hasHTTP && registry != nil {
			defer registry.Unregister(manifest.Name)
		}

		hcDone := make(chan struct{})
		go func() {
			defer close(hcDone)
			runHealthchecks(runCtx, mod.Manifest.Name, sourceModule)
		}()

		err := sourceModule.Run(runCtx)
		cancelRun()
		<-hcDone

		if err != nil && runCtx.Err() == nil {
			log.Printf("modules: source %q run: %v", mod.Manifest.Name, err)
		}
	}()

	return &SourceHandle{
		client: client,
		cancel: cancelRun,
		done:   done,
	}, nil
}

func loadModuleManifest(mod Module) (Manifest, error) {
	manifestPath := filepath.Join(mod.Dir, manifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return mod.Manifest, nil
		}
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	return ParseManifest(data)
}

func runHealthchecks(ctx context.Context, name string, mod SourceModule) {
	ticker := time.NewTicker(healthcheckInterval)
	defer ticker.Stop()

	check := func() {
		resp, err := mod.Healthcheck(ctx)
		if err != nil {
			log.Printf("modules: source %q healthcheck: %v", name, err)
			return
		}
		if resp != nil && !resp.Ok {
			msg := resp.Message
			if msg == "" {
				msg = "unhealthy"
			}
			log.Printf("modules: source %q healthcheck: %s", name, msg)
		}
	}

	check()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}
