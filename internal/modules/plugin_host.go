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

// SourceModule is the host-side client for a running module plugin.
type SourceModule interface {
	Run(ctx context.Context) error
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}

type sourceModuleClient struct {
	client     troverpc.SourceModuleClient
	httpClient troverpc.HTTPModuleClient
	broker     *plugin.GRPCBroker
	journal    journal.Journal
	policy     IngestPolicy
	blobs      blob.Store
	hasHTTP    bool
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
			journal: c.journal,
			policy:  c.policy,
			blobs:   c.blobs,
			query:   querySvc,
		})
		return s
	})

	_, err := c.client.Run(ctx, &troverpc.RunRequest{
		ServicesBrokerId: servicesID,
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

type sourceModuleGRPCPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	journal    journal.Journal
	policy     IngestPolicy
	moduleName string
	blobs      blob.Store
	hasHTTP    bool
}

func (p *sourceModuleGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &sourceModuleClient{
		client:     troverpc.NewSourceModuleClient(c),
		httpClient: troverpc.NewHTTPModuleClient(c),
		broker:     broker,
		journal:    p.journal,
		policy:     p.policy,
		blobs:      p.blobs,
		hasHTTP:    p.hasHTTP,
	}, nil
}

func (p *sourceModuleGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil
}

func hostPluginSet(j journal.Journal, policy IngestPolicy, moduleName string, blobs blob.Store, hasHTTP bool) map[string]plugin.Plugin {
	return map[string]plugin.Plugin{
		trovemodule.PluginName: &sourceModuleGRPCPlugin{
			journal:    j,
			policy:     policy,
			moduleName: moduleName,
			blobs:      blobs,
			hasHTTP:    hasHTTP,
		},
	}
}

var _ plugin.GRPCPlugin = (*sourceModuleGRPCPlugin)(nil)
