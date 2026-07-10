package modules

import (
	"context"

	"github.com/hashicorp/go-plugin"
	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
	"google.golang.org/grpc"
)

// SourceModule is the host-side client for a running source module plugin.
type SourceModule interface {
	Run(ctx context.Context) error
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}

type sourceModuleClient struct {
	client  troverpc.SourceModuleClient
	broker  *plugin.GRPCBroker
	journal journal.Journal
	policy  IngestPolicy
}

func (c *sourceModuleClient) Run(ctx context.Context) error {
	brokerID := c.broker.NextId()
	go c.broker.AcceptAndServe(brokerID, func(opts []grpc.ServerOption) *grpc.Server {
		s := grpc.NewServer(opts...)
		troverpc.RegisterSourceServer(s, &ingestServer{journal: c.journal, policy: c.policy})
		return s
	})

	_, err := c.client.Run(ctx, &troverpc.RunRequest{IngestBrokerId: brokerID})
	return err
}

func (c *sourceModuleClient) Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error) {
	return c.client.Healthcheck(ctx, &troverpc.HealthcheckRequest{})
}

type sourceModuleGRPCPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	journal    journal.Journal
	policy     IngestPolicy
	moduleName string
}

func (p *sourceModuleGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &sourceModuleClient{
		client:  troverpc.NewSourceModuleClient(c),
		broker:  broker,
		journal: p.journal,
		policy:  p.policy,
	}, nil
}

func (p *sourceModuleGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil
}

func hostPluginSet(j journal.Journal, policy IngestPolicy, moduleName string) map[string]plugin.Plugin {
	return map[string]plugin.Plugin{
		trovemodule.PluginName: &sourceModuleGRPCPlugin{
			journal:    j,
			policy:     policy,
			moduleName: moduleName,
		},
	}
}

var _ plugin.GRPCPlugin = (*sourceModuleGRPCPlugin)(nil)
