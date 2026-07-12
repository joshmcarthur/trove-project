package modules

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/joshmcarthur/trove/internal/blob"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/types"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

// CLIRegistry maps CLI command names to discovered modules.
type CLIRegistry struct {
	modules map[string]Module
}

// NewCLIRegistry returns a registry keyed by module name.
func NewCLIRegistry(mods []Module) *CLIRegistry {
	byName := make(map[string]Module, len(mods))
	for _, mod := range mods {
		byName[mod.Manifest.Name] = mod
	}
	return &CLIRegistry{modules: byName}
}

// ModuleForCommand returns the module that owns a CLI command name.
func (r *CLIRegistry) ModuleForCommand(entries []CLICommandEntry, command string) (Module, bool) {
	if r == nil {
		return Module{}, false
	}
	for _, entry := range entries {
		if entry.Command.Name == command {
			mod, ok := r.modules[entry.Module]
			return mod, ok
		}
	}
	return Module{}, false
}

// RunCLICommand starts a module subprocess, invokes its CLI handler, and returns the result.
func RunCLICommand(
	ctx context.Context,
	mod Module,
	catalog *types.Catalog,
	blobs blob.Store,
	command string,
	args []string,
	settings *SettingsStore,
) (stdout, stderr []byte, exitCode int, err error) {
	manifest, err := loadModuleManifest(mod)
	if err != nil {
		return nil, nil, 1, err
	}
	if len(manifest.CLICommands()) == 0 {
		return nil, nil, 1, fmt.Errorf("modules: module %q does not provide CLI commands", manifest.Name)
	}

	policy, err := NewEmitPolicy(nil, catalog, manifest.Name)
	if err != nil {
		return nil, nil, 1, err
	}

	cmd, err := moduleExecCmd(mod, settings)
	if err != nil {
		return nil, nil, 1, err
	}

	caps := moduleCapabilities{
		hasCLI:      true,
		hasMCPTools: len(manifest.MCPTools()) > 0,
		hasHTTP:     len(manifest.HTTPRoutes()) > 0,
	}
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: trovemodule.Handshake,
		Plugins: hostPluginSet(
			nil, policy, manifest.Name, blobs, caps, nil, nil, nil, catalog,
		),
		Cmd:              cmd,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           hclog.NewNullLogger(),
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	if err != nil {
		return nil, nil, 1, fmt.Errorf("modules: cli %q: client: %w", manifest.Name, err)
	}

	raw, err := rpcClient.Dispense(trovemodule.PluginName)
	if err != nil {
		return nil, nil, 1, fmt.Errorf("modules: cli %q: dispense: %w", manifest.Name, err)
	}

	moduleClient, ok := raw.(*moduleClient)
	if !ok {
		return nil, nil, 1, fmt.Errorf("modules: cli %q: unexpected plugin type %T", manifest.Name, raw)
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	runDone := make(chan error, 1)
	go func() {
		runDone <- moduleClient.Run(runCtx)
	}()

	resp, err := moduleClient.RunCommand(ctx, &troverpc.CLICommandRequest{
		Command: command,
		Args:    args,
	})
	cancelRun()
	<-runDone

	if err != nil {
		return nil, nil, 1, err
	}
	return resp.GetStdout(), resp.GetStderr(), int(resp.GetExitCode()), nil
}
