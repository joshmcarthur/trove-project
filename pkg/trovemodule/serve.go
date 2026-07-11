package trovemodule

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-plugin"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"google.golang.org/grpc"
)

// Serve starts a Trove module plugin subprocess. mod must implement Module and
// may also implement HTTPHandler and HealthChecker.
func Serve(mod Module) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			PluginName: &sourceGRPCPlugin{mod: mod},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

type sourceGRPCPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	mod Module
}

func (p *sourceGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return nil, fmt.Errorf("trovemodule: source plugin is server-side only")
}

func (p *sourceGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	troverpc.RegisterSourceModuleServer(s, &sourceModuleServer{
		mod:    p.mod,
		broker: broker,
	})
	if _, ok := p.mod.(HTTPHandler); ok {
		troverpc.RegisterHTTPModuleServer(s, &httpModuleServer{mod: p.mod})
	}
	if _, ok := p.mod.(EventProcessor); ok {
		troverpc.RegisterProcessorModuleServer(s, &processorModuleServer{mod: p.mod})
	}
	if _, ok := p.mod.(EventSink); ok {
		troverpc.RegisterSinkModuleServer(s, &sinkModuleServer{mod: p.mod})
	}
	return nil
}

type sourceModuleServer struct {
	troverpc.UnimplementedSourceModuleServer
	mod    Module
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

	if err := s.mod.Run(ctx, core); err != nil {
		return nil, err
	}
	return &troverpc.RunResponse{}, nil
}

func (s *sourceModuleServer) Healthcheck(ctx context.Context, _ *troverpc.HealthcheckRequest) (*troverpc.HealthcheckResponse, error) {
	if hc, ok := s.mod.(HealthChecker); ok {
		return hc.Healthcheck(ctx)
	}
	return &troverpc.HealthcheckResponse{Ok: true}, nil
}

type httpModuleServer struct {
	troverpc.UnimplementedHTTPModuleServer
	mod Module
}

func (s *httpModuleServer) HandleHTTP(ctx context.Context, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	h, ok := s.mod.(HTTPHandler)
	if !ok {
		return nil, fmt.Errorf("trovemodule: module does not implement HTTPHandler")
	}
	return h.HandleHTTP(ctx, req)
}

type processorModuleServer struct {
	troverpc.UnimplementedProcessorModuleServer
	mod Module
}

func (s *processorModuleServer) Process(ctx context.Context, req *troverpc.ProcessRequest) (*troverpc.ProcessResponse, error) {
	p, ok := s.mod.(EventProcessor)
	if !ok {
		return nil, fmt.Errorf("trovemodule: module does not implement EventProcessor")
	}
	events, err := p.Process(ctx, req.GetEvent(), req.GetContext())
	if err != nil {
		return nil, err
	}
	return &troverpc.ProcessResponse{Events: events}, nil
}

func (s *processorModuleServer) Healthcheck(ctx context.Context, _ *troverpc.HealthcheckRequest) (*troverpc.HealthcheckResponse, error) {
	if hc, ok := s.mod.(HealthChecker); ok {
		return hc.Healthcheck(ctx)
	}
	return &troverpc.HealthcheckResponse{Ok: true}, nil
}

type sinkModuleServer struct {
	troverpc.UnimplementedSinkModuleServer
	mod Module
}

func (s *sinkModuleServer) Handle(ctx context.Context, req *troverpc.HandleRequest) (*troverpc.HandleResponse, error) {
	h, ok := s.mod.(EventSink)
	if !ok {
		return nil, fmt.Errorf("trovemodule: module does not implement EventSink")
	}
	if err := h.Handle(ctx, req.GetEvent(), req.GetContext()); err != nil {
		return nil, err
	}
	return &troverpc.HandleResponse{}, nil
}

func (s *sinkModuleServer) Healthcheck(ctx context.Context, _ *troverpc.HealthcheckRequest) (*troverpc.HealthcheckResponse, error) {
	if hc, ok := s.mod.(HealthChecker); ok {
		return hc.Healthcheck(ctx)
	}
	return &troverpc.HealthcheckResponse{Ok: true}, nil
}

var _ plugin.GRPCPlugin = (*sourceGRPCPlugin)(nil)
