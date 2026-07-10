package trovemodule

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-plugin"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"google.golang.org/grpc"
)

// Serve starts a Trove source module plugin process.
func Serve(runner Runner) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			PluginName: &sourceGRPCPlugin{runner: runner},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

type sourceGRPCPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	runner Runner
}

func (p *sourceGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return nil, fmt.Errorf("trovemodule: source plugin is server-side only")
}

func (p *sourceGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	troverpc.RegisterSourceModuleServer(s, &sourceModuleServer{
		runner: p.runner,
		broker: broker,
	})
	return nil
}

type sourceModuleServer struct {
	troverpc.UnimplementedSourceModuleServer
	runner Runner
	broker *plugin.GRPCBroker
}

func (s *sourceModuleServer) Run(ctx context.Context, req *troverpc.RunRequest) (*troverpc.RunResponse, error) {
	conn, err := s.broker.Dial(req.IngestBrokerId)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	emit := &sourceEmitter{client: troverpc.NewSourceClient(conn)}
	if err := s.runner.Run(ctx, emit); err != nil {
		return nil, err
	}
	return &troverpc.RunResponse{}, nil
}

var _ plugin.GRPCPlugin = (*sourceGRPCPlugin)(nil)
