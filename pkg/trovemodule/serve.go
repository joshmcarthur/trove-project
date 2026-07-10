package trovemodule

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-plugin"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"google.golang.org/grpc"
)

// BlobRunner is implemented by modules that use core blob services.
type BlobRunner interface {
	RunWithBlobs(ctx context.Context, emit Emitter, blobs BlobPutter) error
}

// Serve starts a Trove source module plugin process. runner must implement
// Runner and/or BlobRunner.
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
	var emit Emitter = &discardingEmitter{}
	if req.IngestBrokerId != 0 {
		conn, err := s.broker.Dial(req.IngestBrokerId)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		emit = &sourceEmitter{client: troverpc.NewSourceClient(conn)}
	}

	var blobs BlobPutter
	var querier Querier
	if req.ServicesBrokerId != 0 {
		servicesConn, err := s.broker.Dial(req.ServicesBrokerId)
		if err != nil {
			return nil, err
		}
		defer servicesConn.Close()
		services := troverpc.NewCoreServicesClient(servicesConn)
		blobs = &blobPutter{client: services}
		querier = newCoreQuerier(services)
	}

	switch runner := s.runner.(type) {
	case BlobRunner:
		if err := runner.RunWithBlobs(ctx, emit, blobs); err != nil {
			return nil, err
		}
	case QueryRunner:
		if querier == nil {
			return nil, fmt.Errorf("trovemodule: query services are required")
		}
		if err := runner.RunWithQuery(ctx, querier); err != nil {
			return nil, err
		}
	case Runner:
		if err := runner.Run(ctx, emit); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("trovemodule: runner must implement Runner, BlobRunner, or QueryRunner")
	}
	return &troverpc.RunResponse{}, nil
}

type discardingEmitter struct{}

func (discardingEmitter) Emit(context.Context, *troverpc.Event) error {
	return fmt.Errorf("trovemodule: emit is not available")
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
