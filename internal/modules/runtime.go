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
	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

// healthcheckInterval is the period between module healthchecks.
var healthcheckInterval = 30 * time.Second

// SourceHandle supervises a running source module subprocess.
type SourceHandle struct {
	client *plugin.Client
}

// Close stops the supervised module subprocess.
func (h *SourceHandle) Close() error {
	if h == nil || h.client == nil {
		return nil
	}
	h.client.Kill()
	return nil
}

// StartSource launches mod and routes Emit RPC calls into journal.
// blobsPath is passed to the module subprocess as TROVE_BLOBS_PATH when non-empty.
func StartSource(ctx context.Context, j journal.Journal, mod Module, blobsPath string) (*SourceHandle, error) {
	if mod.Manifest.Kind != KindSource {
		return nil, fmt.Errorf("modules: start source: %q is kind %q, want %q", mod.Manifest.Name, mod.Manifest.Kind, KindSource)
	}

	manifest, err := loadModuleManifest(mod)
	if err != nil {
		return nil, fmt.Errorf("modules: start source %q: %w", mod.Manifest.Name, err)
	}

	policy, err := LoadIngestPolicy(manifest, mod.Dir)
	if err != nil {
		return nil, fmt.Errorf("modules: start source %q: %w", mod.Manifest.Name, err)
	}

	cmd, err := moduleExecCmd(mod, blobsPath)
	if err != nil {
		return nil, fmt.Errorf("modules: start source %q: %w", mod.Manifest.Name, err)
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  trovemodule.Handshake,
		Plugins:          hostPluginSet(j, policy, manifest.Name),
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

	sourceModule, ok := raw.(SourceModule)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("modules: start source %q: unexpected plugin type %T", mod.Manifest.Name, raw)
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	hcDone := make(chan struct{})
	go func() {
		defer close(hcDone)
		runHealthchecks(runCtx, mod.Manifest.Name, sourceModule)
	}()

	err = sourceModule.Run(runCtx)
	cancelRun()
	<-hcDone

	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("modules: start source %q: run: %w", mod.Manifest.Name, err)
	}

	return &SourceHandle{client: client}, nil
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
