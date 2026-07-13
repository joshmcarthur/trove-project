package trovemodule

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/go-plugin"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// Core is the module's connection to the Trove parent process. Modules append and
// query revisions, store blobs, invoke MCP tools, and introspect types.
type Core interface {
	RevisionAppender
	RevisionQuerier
	BlobPutter
	MCPToolCaller
	TypeCatalogReader
}

// Module is the main entry contract for trovemodule.Serve. Implement Run to
// receive a Core handle when the parent starts the module.
type Module interface {
	Run(ctx context.Context, core Core) error
}

// RunFunc adapts a function to Module.
type RunFunc func(ctx context.Context, core Core) error

func (f RunFunc) Run(ctx context.Context, core Core) error {
	return f(ctx, core)
}

func connectCore(broker *plugin.GRPCBroker, handle uint32) (Core, error) {
	if handle == 0 {
		return nil, errCoreUnavailable
	}
	conn, err := broker.Dial(handle)
	if err != nil {
		return nil, err
	}
	return &coreConn{
		client: troverpc.NewCoreServicesClient(conn),
		closer: conn,
	}, nil
}

type coreConn struct {
	client troverpc.CoreServicesClient
	closer interface{ Close() error }
}

func (c *coreConn) Close() error {
	return c.closer.Close()
}

func (c *coreConn) AppendRevision(ctx context.Context, req *troverpc.AppendRevisionRequest) (*troverpc.AppendRevisionResponse, error) {
	return c.client.AppendRevision(ctx, req)
}

func (c *coreConn) Put(ctx context.Context, data []byte) (string, error) {
	resp, err := c.client.BlobPut(ctx, &troverpc.BlobPutRequest{Data: data})
	if err != nil {
		return "", err
	}
	return resp.GetBlobRef(), nil
}

func (c *coreConn) GetRevision(ctx context.Context, id string) (*troverpc.Revision, error) {
	return c.client.GetRevision(ctx, &troverpc.GetRevisionRequest{Id: id})
}

func (c *coreConn) SearchRevisions(ctx context.Context, req *troverpc.SearchRevisionsRequest) ([]*troverpc.Revision, error) {
	resp, err := c.client.SearchRevisions(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetRevisions(), nil
}

func (c *coreConn) GetRevisionsByType(ctx context.Context, req *troverpc.GetRevisionsByTypeRequest) ([]*troverpc.Revision, error) {
	resp, err := c.client.GetRevisionsByType(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetRevisions(), nil
}

func (c *coreConn) SummarizeRange(ctx context.Context, req *troverpc.SummarizeRangeRequest) (*troverpc.Summary, error) {
	return c.client.SummarizeRange(ctx, req)
}

func (c *coreConn) ListMCPTools(ctx context.Context) ([]MCPToolDescriptor, error) {
	resp, err := c.client.ListMCPTools(ctx, &troverpc.ListMCPToolsRequest{})
	if err != nil {
		return nil, err
	}
	tools := make([]MCPToolDescriptor, 0, len(resp.GetTools()))
	for _, tool := range resp.GetTools() {
		tools = append(tools, MCPToolDescriptor{
			Name:        tool.GetName(),
			Description: tool.GetDescription(),
			Module:      tool.GetModule(),
		})
	}
	return tools, nil
}

func (c *coreConn) CallMCPTool(ctx context.Context, name string, arguments json.RawMessage) (json.RawMessage, error) {
	resp, err := c.client.CallMCPTool(ctx, &troverpc.MCPToolCallRequest{
		Name:          name,
		ArgumentsJson: arguments,
	})
	if err != nil {
		return nil, err
	}
	if resp.GetIsError() {
		msg := resp.GetMessage()
		if msg == "" {
			msg = "mcp tool call failed"
		}
		return nil, &mcpToolError{msg: msg}
	}
	return resp.GetResultJson(), nil
}

func (c *coreConn) ListTypes(ctx context.Context, sourceFilter string) ([]TypeSummary, error) {
	resp, err := c.client.ListTypes(ctx, &troverpc.ListTypesRequest{SourceFilter: sourceFilter})
	if err != nil {
		return nil, err
	}
	return protoTypeSummaries(resp.GetTypes()), nil
}

func (c *coreConn) GetType(ctx context.Context, uri string) (TypeSummary, json.RawMessage, error) {
	resp, err := c.client.GetType(ctx, &troverpc.GetTypeRequest{Uri: uri})
	if err != nil {
		return TypeSummary{}, nil, err
	}
	return protoTypeSummary(resp.GetSummary()), json.RawMessage(resp.GetDefinitionJson()), nil
}

func (c *coreConn) ExportType(ctx context.Context, uri string) ([]byte, string, error) {
	resp, err := c.client.ExportType(ctx, &troverpc.ExportTypeRequest{Uri: uri})
	if err != nil {
		return nil, "", err
	}
	return resp.GetTtdJson(), resp.GetSchemaRef(), nil
}

func (c *coreConn) ValidateTypeDefinition(ctx context.Context, ttdJSON []byte) (bool, string, string, error) {
	resp, err := c.client.ValidateTypeDefinition(ctx, &troverpc.ValidateTypeDefinitionRequest{TtdJson: ttdJSON})
	if err != nil {
		return false, "", "", err
	}
	return resp.GetValid(), resp.GetUri(), resp.GetError(), nil
}

func protoTypeSummaries(in []*troverpc.TypeSummary) []TypeSummary {
	out := make([]TypeSummary, 0, len(in))
	for _, s := range in {
		out = append(out, protoTypeSummary(s))
	}
	return out
}

func protoTypeSummary(s *troverpc.TypeSummary) TypeSummary {
	if s == nil {
		return TypeSummary{}
	}
	return TypeSummary{
		URI:         s.GetUri(),
		Title:       s.GetTitle(),
		Description: s.GetDescription(),
		Source:      s.GetSource(),
		SourcePath:  s.GetSourcePath(),
		SchemaRef:   s.GetSchemaRef(),
		Status:      s.GetStatus(),
	}
}

type mcpToolError struct {
	msg string
}

func (e *mcpToolError) Error() string {
	return e.msg
}
