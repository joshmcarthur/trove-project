package trovemodule

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/go-plugin"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// Core is the module's connection to the Trove parent process. Use it to append
// events, store blobs, read the journal, invoke module MCP tools, and introspect types.
type Core interface {
	RecordWriter
	BlobPutter
	Querier
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
	// Connection lifetime is bound to Run; closed when Run returns.
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

func (c *coreConn) RecordWrite(ctx context.Context, req *troverpc.WriteRequest) (*troverpc.WriteResponse, error) {
	return c.client.RecordWrite(ctx, req)
}

func (c *coreConn) Put(ctx context.Context, data []byte) (string, error) {
	resp, err := c.client.BlobPut(ctx, &troverpc.BlobPutRequest{Data: data})
	if err != nil {
		return "", err
	}
	return resp.GetBlobRef(), nil
}

func (c *coreConn) GetEvent(ctx context.Context, id string) (*troverpc.Event, error) {
	return c.client.GetEvent(ctx, &troverpc.GetEventRequest{Id: id})
}

func (c *coreConn) SearchEvents(ctx context.Context, req *troverpc.SearchEventsRequest) ([]*troverpc.Event, error) {
	resp, err := c.client.SearchEvents(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetEvents(), nil
}

func (c *coreConn) GetEventsByType(ctx context.Context, req *troverpc.GetEventsByTypeRequest) ([]*troverpc.Event, error) {
	resp, err := c.client.GetEventsByType(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetEvents(), nil
}

func (c *coreConn) SummarizeRange(ctx context.Context, req *troverpc.SummarizeRangeRequest) (*troverpc.Summary, error) {
	return c.client.SummarizeRange(ctx, req)
}

func (c *coreConn) GetRecord(ctx context.Context, req *troverpc.GetRecordRequest) (*troverpc.Record, error) {
	return c.client.GetRecord(ctx, req)
}

func (c *coreConn) SearchRecords(ctx context.Context, req *troverpc.SearchRecordsRequest) ([]*troverpc.Record, error) {
	resp, err := c.client.SearchRecords(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetRecords(), nil
}

func (c *coreConn) ListIncompleteRecords(ctx context.Context, req *troverpc.ListIncompleteRecordsRequest) ([]*troverpc.Record, error) {
	resp, err := c.client.ListIncompleteRecords(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetRecords(), nil
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
