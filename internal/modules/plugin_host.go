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

// SourceModule is the host-side client for a running module plugin.
type SourceModule interface {
	Run(ctx context.Context) error
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}

type sourceModuleClient struct {
	client      troverpc.SourceModuleClient
	httpClient  troverpc.HTTPModuleClient
	mcpClient   troverpc.MCPModuleClient
	broker      *plugin.GRPCBroker
	journal     journal.Journal
	policy      IngestPolicy
	blobs       blob.Store
	mcpTools    []MCPToolEntry
	toolModules map[string]string
	mcpRegistry *MCPRegistry
	hasHTTP     bool
	hasMCPTools bool
}

func (c *sourceModuleClient) Run(ctx context.Context) error {
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

	_, err := c.client.Run(ctx, &troverpc.RunRequest{
		ServicesBrokerId: servicesID, // parent-process handle for CoreServices
	})
	return err
}

func (c *sourceModuleClient) Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error) {
	return c.client.Healthcheck(ctx, &troverpc.HealthcheckRequest{})
}

func (c *sourceModuleClient) HandleHTTP(ctx context.Context, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	if !c.hasHTTP {
		return nil, errHTTPNotSupported
	}
	return c.httpClient.HandleHTTP(ctx, req)
}

func (c *sourceModuleClient) CallTool(ctx context.Context, req *troverpc.MCPToolCallRequest) (*troverpc.MCPToolCallResponse, error) {
	if !c.hasMCPTools {
		return nil, errMCPNotSupported
	}
	return c.mcpClient.CallTool(ctx, req)
}

type sourceModuleGRPCPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	journal     journal.Journal
	policy      IngestPolicy
	moduleName  string
	blobs       blob.Store
	mcpTools    []MCPToolEntry
	toolModules map[string]string
	mcpRegistry *MCPRegistry
	hasHTTP     bool
	hasMCPTools bool
}

func (p *sourceModuleGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &sourceModuleClient{
		client:      troverpc.NewSourceModuleClient(c),
		httpClient:  troverpc.NewHTTPModuleClient(c),
		mcpClient:   troverpc.NewMCPModuleClient(c),
		broker:      broker,
		journal:     p.journal,
		policy:      p.policy,
		blobs:       p.blobs,
		mcpTools:    p.mcpTools,
		toolModules: p.toolModules,
		mcpRegistry: p.mcpRegistry,
		hasHTTP:     p.hasHTTP,
		hasMCPTools: p.hasMCPTools,
	}, nil
}

func (p *sourceModuleGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil
}

func hostPluginSet(
	j journal.Journal,
	policy IngestPolicy,
	moduleName string,
	blobs blob.Store,
	hasHTTP bool,
	hasMCPTools bool,
	mcpTools []MCPToolEntry,
	toolModules map[string]string,
	mcpRegistry *MCPRegistry,
) map[string]plugin.Plugin {
	return map[string]plugin.Plugin{
		trovemodule.PluginName: &sourceModuleGRPCPlugin{
			journal:     j,
			policy:      policy,
			moduleName:  moduleName,
			blobs:       blobs,
			mcpTools:    mcpTools,
			toolModules: toolModules,
			mcpRegistry: mcpRegistry,
			hasHTTP:     hasHTTP,
			hasMCPTools: hasMCPTools,
		},
	}
}

var _ plugin.GRPCPlugin = (*sourceModuleGRPCPlugin)(nil)
