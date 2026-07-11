package trovemodule

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-plugin"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"google.golang.org/grpc"
)

// Serve starts a Trove module plugin subprocess. runner must implement Module
// and may also implement HTTPHandler and HealthChecker.
func Serve(runner any) {
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
	runner any
}

func (p *sourceGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return nil, fmt.Errorf("trovemodule: source plugin is server-side only")
}

func (p *sourceGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	troverpc.RegisterSourceModuleServer(s, &sourceModuleServer{
		runner: p.runner,
		broker: broker,
	})
	if _, ok := p.runner.(HTTPHandler); ok {
		troverpc.RegisterHTTPModuleServer(s, &httpModuleServer{runner: p.runner})
	}
	return nil
}

type sourceModuleServer struct {
	troverpc.UnimplementedSourceModuleServer
	runner any
	broker *plugin.GRPCBroker
}

func (s *sourceModuleServer) Run(ctx context.Context, req *troverpc.RunRequest) (*troverpc.RunResponse, error) {
	core, err := connectCore(s.broker, req.ServicesBrokerId)
	if err != nil {
		return nil, err
	}
	if c, ok := core.(*coreConn); ok {
		defer c.Close()
	}

	if err := runModule(ctx, s.runner, core); err != nil {
		return nil, err
	}
	return &troverpc.RunResponse{}, nil
}

func runModule(ctx context.Context, runner any, core Core) error {
	switch mod := runner.(type) {
	case Module:
		return mod.Run(ctx, core)
	case BlobRunner:
		return mod.RunWithBlobs(ctx, core, core)
	case QueryRunner:
		return mod.RunWithQuery(ctx, core)
	case Runner:
		return mod.Run(ctx, core)
	default:
		return fmt.Errorf("trovemodule: runner must implement Module")
	}
}

func (s *sourceModuleServer) Healthcheck(ctx context.Context, _ *troverpc.HealthcheckRequest) (*troverpc.HealthcheckResponse, error) {
	if hc, ok := s.runner.(HealthChecker); ok {
		return hc.Healthcheck(ctx)
	}
	return &troverpc.HealthcheckResponse{Ok: true}, nil
}

type httpModuleServer struct {
	troverpc.UnimplementedHTTPModuleServer
	runner any
}

func (s *httpModuleServer) HandleHTTP(ctx context.Context, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	h, ok := s.runner.(HTTPHandler)
	if !ok {
		return nil, fmt.Errorf("trovemodule: module does not implement HTTPHandler")
	}
	return h.HandleHTTP(ctx, req)
}

var _ plugin.GRPCPlugin = (*sourceGRPCPlugin)(nil)
