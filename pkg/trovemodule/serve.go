package trovemodule

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-plugin"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"google.golang.org/grpc"
)

// Serve starts a Trove module plugin subprocess. mod must implement Module and
// may also implement HTTPHandler, AuthHandler, MCPToolHandler, CLIHandler,
// RevisionProcessor, RevisionSink, and HealthChecker.
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
	if _, ok := p.mod.(AuthHandler); ok {
		troverpc.RegisterAuthModuleServer(s, &authModuleServer{mod: p.mod})
	}
	if _, ok := p.mod.(MCPToolHandler); ok {
		troverpc.RegisterMCPModuleServer(s, &mcpModuleServer{mod: p.mod})
	}
	if _, ok := p.mod.(CLIHandler); ok {
		troverpc.RegisterCLIModuleServer(s, &cliModuleServer{mod: p.mod})
	}
	if _, ok := p.mod.(RevisionProcessor); ok {
		troverpc.RegisterProcessorModuleServer(s, &processorModuleServer{mod: p.mod})
	}
	if _, ok := p.mod.(RevisionSink); ok {
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

type authModuleServer struct {
	troverpc.UnimplementedAuthModuleServer
	mod Module
}

func (s *authModuleServer) ValidateAuth(ctx context.Context, req *troverpc.AuthRequest) (*troverpc.AuthResponse, error) {
	h, ok := s.mod.(AuthHandler)
	if !ok {
		return nil, fmt.Errorf("trovemodule: module does not implement AuthHandler")
	}
	return h.ValidateAuth(ctx, req)
}

type mcpModuleServer struct {
	troverpc.UnimplementedMCPModuleServer
	mod Module
}

func (s *mcpModuleServer) CallTool(ctx context.Context, req *troverpc.MCPToolCallRequest) (*troverpc.MCPToolCallResponse, error) {
	h, ok := s.mod.(MCPToolHandler)
	if !ok {
		return nil, fmt.Errorf("trovemodule: module does not implement MCPToolHandler")
	}
	result, err := h.CallTool(ctx, req.GetName(), req.GetArgumentsJson())
	if err != nil {
		return &troverpc.MCPToolCallResponse{
			IsError: true,
			Message: err.Error(),
		}, nil
	}
	return &troverpc.MCPToolCallResponse{ResultJson: result}, nil
}

type cliModuleServer struct {
	troverpc.UnimplementedCLIModuleServer
	mod Module
}

func (s *cliModuleServer) RunCommand(ctx context.Context, req *troverpc.CLICommandRequest) (*troverpc.CLICommandResponse, error) {
	h, ok := s.mod.(CLIHandler)
	if !ok {
		return nil, fmt.Errorf("trovemodule: module does not implement CLIHandler")
	}
	return RunCommandRPC(ctx, h, req)
}

type processorModuleServer struct {
	troverpc.UnimplementedProcessorModuleServer
	mod Module
}

func (s *processorModuleServer) Process(ctx context.Context, req *troverpc.ProcessRequest) (*troverpc.ProcessResponse, error) {
	p, ok := s.mod.(RevisionProcessor)
	if !ok {
		return nil, fmt.Errorf("trovemodule: module does not implement RevisionProcessor")
	}
	events, err := p.Process(ctx, req.GetRevision(), req.GetContext())
	if err != nil {
		return nil, err
	}
	return &troverpc.ProcessResponse{Revisions: events}, nil
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
	h, ok := s.mod.(RevisionSink)
	if !ok {
		return nil, fmt.Errorf("trovemodule: module does not implement RevisionSink")
	}
	if err := h.Handle(ctx, req.GetRevision(), req.GetContext()); err != nil {
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
