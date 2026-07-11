package modules

import (
	"context"
	"errors"

	"github.com/hashicorp/go-plugin"
	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/query"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
	"google.golang.org/grpc"
)

var errHTTPNotSupported = errors.New("modules: module does not support HTTP")
var errMCPNotSupported = errors.New("modules: module does not support MCP tools")

type moduleCapabilities struct {
	hasHTTP      bool
	hasProcessor bool
	hasSink      bool
	hasMCPTools  bool
	needsSource  bool
}

// ModuleHandle supervises a running module subprocess.
type ModuleHandle struct {
	client *plugin.Client
	cancel context.CancelFunc
	done   chan struct{}
}

// Close stops the supervised module subprocess.
func (h *ModuleHandle) Close() error {
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

// SourceModule is the host-side client for a running module plugin.
type SourceModule interface {
	Run(ctx context.Context) error
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}

type moduleClient struct {
	sourceClient    troverpc.SourceModuleClient
	processorClient troverpc.ProcessorModuleClient
	sinkClient      troverpc.SinkModuleClient
	httpClient      troverpc.HTTPModuleClient
	mcpClient       troverpc.MCPModuleClient
	broker          *plugin.GRPCBroker
	journal         journal.Journal
	policy          IngestPolicy
	blobs           blob.Store
	mcpTools        []MCPToolEntry
	toolModules     map[string]string
	mcpRegistry     *MCPRegistry
	caps            moduleCapabilities
}

func (c *moduleClient) Run(ctx context.Context) error {
	if !c.caps.needsSource {
		<-ctx.Done()
		return ctx.Err()
	}

	servicesID := c.broker.NextId()
	go c.broker.AcceptAndServe(servicesID, func(opts []grpc.ServerOption) *grpc.Server {
		s := grpc.NewServer(opts...)
		var querySvc *query.Service
		if c.journal != nil {
			querySvc = &query.Service{Journal: c.journal}
		}
		troverpc.RegisterCoreServicesServer(s, &coreServicesServer{
			journal:     c.journal,
			policy:      c.policy,
			blobs:       c.blobs,
			query:       querySvc,
			mcpTools:    c.mcpTools,
			toolModules: c.toolModules,
			mcpRegistry: c.mcpRegistry,
		})
		return s
	})

	_, err := c.sourceClient.Run(ctx, &troverpc.RunRequest{
		ServicesBrokerId: servicesID,
	})
	return err
}

func (c *moduleClient) Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error) {
	switch {
	case c.caps.hasProcessor:
		return c.processorClient.Healthcheck(ctx, &troverpc.HealthcheckRequest{})
	case c.caps.hasSink:
		return c.sinkClient.Healthcheck(ctx, &troverpc.HealthcheckRequest{})
	default:
		return c.sourceClient.Healthcheck(ctx, &troverpc.HealthcheckRequest{})
	}
}

func (c *moduleClient) HandleHTTP(ctx context.Context, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	if !c.caps.hasHTTP {
		return nil, errHTTPNotSupported
	}
	return c.httpClient.HandleHTTP(ctx, req)
}

func (c *moduleClient) CallTool(ctx context.Context, req *troverpc.MCPToolCallRequest) (*troverpc.MCPToolCallResponse, error) {
	if !c.caps.hasMCPTools {
		return nil, errMCPNotSupported
	}
	return c.mcpClient.CallTool(ctx, req)
}

func (c *moduleClient) Process(ctx context.Context, event journal.Event, dispatch DispatchContext) ([]journal.Event, error) {
	if !c.caps.hasProcessor {
		return nil, errors.New("modules: module does not support Process")
	}
	resp, err := c.processorClient.Process(ctx, &troverpc.ProcessRequest{
		Event:   journalEventToRPC(event),
		Context: dispatchContextToProto(dispatch),
	})
	if err != nil {
		return nil, err
	}
	out := make([]journal.Event, 0, len(resp.GetEvents()))
	for _, e := range resp.GetEvents() {
		converted, err := rpcEventToJournal(e)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func (c *moduleClient) Handle(ctx context.Context, event journal.Event, dispatch DispatchContext) error {
	if !c.caps.hasSink {
		return errors.New("modules: module does not support Handle")
	}
	_, err := c.sinkClient.Handle(ctx, &troverpc.HandleRequest{
		Event:   journalEventToRPC(event),
		Context: dispatchContextToProto(dispatch),
	})
	return err
}

type moduleGRPCPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	journal     journal.Journal
	policy      IngestPolicy
	moduleName  string
	blobs       blob.Store
	mcpTools    []MCPToolEntry
	toolModules map[string]string
	mcpRegistry *MCPRegistry
	caps        moduleCapabilities
}

func (p *moduleGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &moduleClient{
		sourceClient:    troverpc.NewSourceModuleClient(c),
		processorClient: troverpc.NewProcessorModuleClient(c),
		sinkClient:      troverpc.NewSinkModuleClient(c),
		httpClient:      troverpc.NewHTTPModuleClient(c),
		mcpClient:       troverpc.NewMCPModuleClient(c),
		broker:          broker,
		journal:         p.journal,
		policy:          p.policy,
		blobs:           p.blobs,
		mcpTools:        p.mcpTools,
		toolModules:     p.toolModules,
		mcpRegistry:     p.mcpRegistry,
		caps:            p.caps,
	}, nil
}

func (p *moduleGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil
}

func hostPluginSet(
	j journal.Journal,
	policy IngestPolicy,
	moduleName string,
	blobs blob.Store,
	caps moduleCapabilities,
	mcpTools []MCPToolEntry,
	toolModules map[string]string,
	mcpRegistry *MCPRegistry,
) map[string]plugin.Plugin {
	return map[string]plugin.Plugin{
		trovemodule.PluginName: &moduleGRPCPlugin{
			journal:     j,
			policy:      policy,
			moduleName:  moduleName,
			blobs:       blobs,
			mcpTools:    mcpTools,
			toolModules: toolModules,
			mcpRegistry: mcpRegistry,
			caps:        caps,
		},
	}
}

var _ plugin.GRPCPlugin = (*moduleGRPCPlugin)(nil)
