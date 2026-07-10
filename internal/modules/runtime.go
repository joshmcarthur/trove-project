package modules

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

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
func StartSource(ctx context.Context, j journal.Journal, mod Module) (*SourceHandle, error) {
	if mod.Manifest.Kind != KindSource {
		return nil, fmt.Errorf("modules: start source: %q is kind %q, want %q", mod.Manifest.Name, mod.Manifest.Kind, KindSource)
	}

	cmd, err := moduleExecCmd(mod)
	if err != nil {
		return nil, fmt.Errorf("modules: start source %q: %w", mod.Manifest.Name, err)
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  trovemodule.Handshake,
		Plugins:          hostPluginSet(j),
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

	if err := sourceModule.Run(ctx); err != nil {
		client.Kill()
		return nil, fmt.Errorf("modules: start source %q: run: %w", mod.Manifest.Name, err)
	}

	return &SourceHandle{client: client}, nil
}
