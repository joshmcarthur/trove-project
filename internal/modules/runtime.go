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
type SourceHandle = ModuleHandle

// StartSource launches a supervised module subprocess.
func StartSource(
	ctx context.Context,
	j journal.Journal,
	mod Module,
	blobs blob.Store,
	httpRegistry *HTTPRegistry,
	mcpRegistry *MCPRegistry,
	eventRegistry *EventRegistry,
	mcpTools []MCPToolEntry,
	toolModules map[string]string,
	settings *SettingsStore,
) (*SourceHandle, error) {
	manifest, err := loadModuleManifest(mod)
	if err != nil {
		return nil, fmt.Errorf("modules: start %q: %w", mod.Manifest.Name, err)
	}

	hasHTTP := len(manifest.HTTPRoutes()) > 0
	hasMCPTools := len(manifest.MCPTools()) > 0
	needsSource := manifest.Kind == KindSource
	hasProcessor := manifest.Kind == KindProcessor && manifest.EventRoutes()
	hasSink := manifest.Kind == KindSink && manifest.EventRoutes()

	switch {
	case needsSource:
	case hasHTTP:
	case hasMCPTools:
	case hasProcessor:
	case hasSink:
	default:
		return nil, fmt.Errorf("modules: start %q: not a supported module type", mod.Manifest.Name)
	}

	policy, err := LoadIngestPolicy(manifest, mod.Dir)
	if err != nil {
		return nil, fmt.Errorf("modules: start %q: %w", mod.Manifest.Name, err)
	}

	cmd, err := moduleExecCmd(mod, settings)
	if err != nil {
		return nil, fmt.Errorf("modules: start %q: %w", mod.Manifest.Name, err)
	}

	caps := moduleCapabilities{
		hasHTTP:      hasHTTP,
		hasProcessor: hasProcessor,
		hasSink:      hasSink,
		hasMCPTools:  hasMCPTools,
		needsSource:  needsSource,
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: trovemodule.Handshake,
		Plugins: hostPluginSet(
			j, policy, manifest.Name, blobs, caps, mcpTools, toolModules, mcpRegistry,
		),
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

	moduleClient, ok := raw.(*moduleClient)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("modules: start source %q: unexpected plugin type %T", mod.Manifest.Name, raw)
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	done := make(chan struct{})

	if hasHTTP && httpRegistry != nil {
		httpRegistry.Register(manifest.Name, moduleClient)
	}
	if hasMCPTools && mcpRegistry != nil {
		mcpRegistry.Register(manifest.Name, moduleClient)
	}
	if hasProcessor && eventRegistry != nil {
		eventRegistry.RegisterProcessor(manifest.Name, manifest.Consumes, policy, moduleClient)
	}
	if hasSink && eventRegistry != nil {
		eventRegistry.RegisterSink(manifest.Name, manifest.Consumes, moduleClient)
	}

	go func() {
		defer close(done)
		if hasHTTP && httpRegistry != nil {
			defer httpRegistry.Unregister(manifest.Name)
		}
		if hasMCPTools && mcpRegistry != nil {
			defer mcpRegistry.Unregister(manifest.Name)
		}
		if hasProcessor && eventRegistry != nil {
			defer eventRegistry.UnregisterProcessor(manifest.Name)
		}
		if hasSink && eventRegistry != nil {
			defer eventRegistry.UnregisterSink(manifest.Name)
		}

		hcDone := make(chan struct{})
		go func() {
			defer close(hcDone)
			runHealthchecks(runCtx, mod.Manifest.Name, moduleClient)
		}()

		err := moduleClient.Run(runCtx)
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
