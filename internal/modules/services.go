package modules

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/query"
	"github.com/joshmcarthur/trove/internal/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type coreServicesServer struct {
	troverpc.UnimplementedCoreServicesServer
	journal     journal.Journal
	store       *journal.Store
	policy      WritePolicy
	writer      *WriteService
	blobs       blob.Store
	catalog     *types.Catalog
	query       *query.Service
	records     *query.RecordService
	mcpTools    []MCPToolEntry
	toolModules map[string]string
	mcpRegistry *MCPRegistry
}

func (s *coreServicesServer) RecordWrite(ctx context.Context, req *troverpc.WriteRequest) (*troverpc.WriteResponse, error) {
	if s.writer == nil {
		return nil, status.Error(codes.Unavailable, "record write is not configured")
	}
	resp, err := s.writer.WriteFromRPC(ctx, req, s.policy)
	if err != nil {
		return nil, writeGRPCError(err)
	}
	return resp, nil
}

func (s *coreServicesServer) BlobPut(ctx context.Context, req *troverpc.BlobPutRequest) (*troverpc.BlobPutResponse, error) {
	if s.blobs == nil {
		return nil, status.Error(codes.Unavailable, "blob store is not configured")
	}
	if req == nil || len(req.Data) == 0 {
		return nil, status.Error(codes.InvalidArgument, "blob data is required")
	}
	ref, err := s.blobs.Put(ctx, bytes.NewReader(req.Data))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &troverpc.BlobPutResponse{BlobRef: ref}, nil
}

func (s *coreServicesServer) GetEvent(ctx context.Context, req *troverpc.GetEventRequest) (*troverpc.Event, error) {
	if s.query == nil {
		return nil, status.Error(codes.Unavailable, "query is not configured")
	}
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	event, err := s.query.GetEvent(ctx, req.Id)
	if err != nil {
		return nil, queryGRPCError(err)
	}
	return queryEventToProto(event), nil
}

func (s *coreServicesServer) SearchEvents(ctx context.Context, req *troverpc.SearchEventsRequest) (*troverpc.SearchEventsResponse, error) {
	if s.query == nil {
		return nil, status.Error(codes.Unavailable, "query is not configured")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	timeFrom, err := parseOptionalProtoTime(req.TimeFrom)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	timeTo, err := parseOptionalProtoTime(req.TimeTo)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	events, err := s.query.SearchEvents(ctx, req.Query, query.SearchParams{
		TypePrefix: req.TypePrefix,
		Source:     req.Source,
		TimeFrom:   timeFrom,
		TimeTo:     timeTo,
	})
	if err != nil {
		return nil, queryGRPCError(err)
	}
	return &troverpc.SearchEventsResponse{Events: queryEventsToProto(events)}, nil
}

func (s *coreServicesServer) GetRecord(ctx context.Context, req *troverpc.GetRecordRequest) (*troverpc.Record, error) {
	if s.records == nil {
		return nil, status.Error(codes.Unavailable, "record query is not configured")
	}
	if req == nil || req.RecordRef == "" {
		return nil, status.Error(codes.InvalidArgument, "record_ref is required")
	}
	rec, err := s.records.GetRecord(ctx, req.RecordRef, int(req.Version))
	if err != nil {
		return nil, queryGRPCError(err)
	}
	return queryRecordToProto(rec), nil
}

func (s *coreServicesServer) SearchRecords(ctx context.Context, req *troverpc.SearchRecordsRequest) (*troverpc.SearchRecordsResponse, error) {
	if s.records == nil {
		return nil, status.Error(codes.Unavailable, "record query is not configured")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	timeFrom, err := parseOptionalProtoTime(req.TimeFrom)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	timeTo, err := parseOptionalProtoTime(req.TimeTo)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	records, err := s.records.SearchRecords(ctx, req.Query, query.RecordSearchParams{
		TypePrefix:     req.TypePrefix,
		Source:         req.Source,
		TimeFrom:       timeFrom,
		TimeTo:         timeTo,
		IncludeDeleted: req.IncludeDeleted,
	})
	if err != nil {
		return nil, queryGRPCError(err)
	}
	return &troverpc.SearchRecordsResponse{Records: queryRecordsToProto(records)}, nil
}

func (s *coreServicesServer) ListIncompleteRecords(ctx context.Context, req *troverpc.ListIncompleteRecordsRequest) (*troverpc.ListIncompleteRecordsResponse, error) {
	if s.records == nil {
		return nil, status.Error(codes.Unavailable, "record query is not configured")
	}
	if req == nil {
		req = &troverpc.ListIncompleteRecordsRequest{}
	}
	records, err := s.records.ListIncompleteRecords(ctx, req.Source, int(req.Limit))
	if err != nil {
		return nil, queryGRPCError(err)
	}
	return &troverpc.ListIncompleteRecordsResponse{Records: queryRecordsToProto(records)}, nil
}

func (s *coreServicesServer) GetEventsByType(ctx context.Context, req *troverpc.GetEventsByTypeRequest) (*troverpc.SearchEventsResponse, error) {
	if s.query == nil {
		return nil, status.Error(codes.Unavailable, "query is not configured")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	timeFrom, err := parseOptionalProtoTime(req.TimeFrom)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	timeTo, err := parseOptionalProtoTime(req.TimeTo)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	events, err := s.query.GetEventsByType(ctx, req.Type, timeFrom, timeTo)
	if err != nil {
		return nil, queryGRPCError(err)
	}
	return &troverpc.SearchEventsResponse{Events: queryEventsToProto(events)}, nil
}

func (s *coreServicesServer) SummarizeRange(ctx context.Context, req *troverpc.SummarizeRangeRequest) (*troverpc.Summary, error) {
	if s.query == nil {
		return nil, status.Error(codes.Unavailable, "query is not configured")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	timeFrom, err := time.Parse(time.RFC3339, req.TimeFrom)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid time_from: %v", err)
	}
	timeTo, err := time.Parse(time.RFC3339, req.TimeTo)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid time_to: %v", err)
	}
	summary, err := s.query.SummarizeRange(ctx, timeFrom, timeTo)
	if err != nil {
		return nil, queryGRPCError(err)
	}
	return summaryToProto(summary), nil
}

func (s *coreServicesServer) ListMCPTools(ctx context.Context, _ *troverpc.ListMCPToolsRequest) (*troverpc.ListMCPToolsResponse, error) {
	_ = ctx
	tools := make([]*troverpc.MCPToolDescriptor, 0, len(s.mcpTools))
	for _, entry := range s.mcpTools {
		tools = append(tools, &troverpc.MCPToolDescriptor{
			Name:        entry.Tool.Name,
			Description: entry.Tool.Description,
			Module:      entry.Module,
		})
	}
	return &troverpc.ListMCPToolsResponse{Tools: tools}, nil
}

func (s *coreServicesServer) CallMCPTool(ctx context.Context, req *troverpc.MCPToolCallRequest) (*troverpc.MCPToolCallResponse, error) {
	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "tool name is required")
	}
	if s.mcpRegistry == nil {
		return nil, status.Error(codes.Unavailable, "mcp registry is not configured")
	}
	module, ok := s.toolModules[req.Name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "mcp tool %q not found", req.Name)
	}
	dispatcher, ok := s.mcpRegistry.Get(module)
	if !ok {
		return nil, status.Errorf(codes.Unavailable, "mcp module %q is not available", module)
	}
	return dispatcher.CallTool(ctx, req)
}

func (s *coreServicesServer) ListTypes(ctx context.Context, req *troverpc.ListTypesRequest) (*troverpc.ListTypesResponse, error) {
	_ = ctx
	if s.catalog == nil {
		return nil, status.Error(codes.Unavailable, "type catalog is not configured")
	}
	filter := ""
	if req != nil {
		filter = req.GetSourceFilter()
	}
	summaries := make([]*troverpc.TypeSummary, 0)
	for _, entry := range s.catalog.List() {
		summary := typeSummaryFromEntry(entry)
		if filter != "" && summary.Source != filter {
			continue
		}
		summaries = append(summaries, summary)
	}
	return &troverpc.ListTypesResponse{Types: summaries}, nil
}

func (s *coreServicesServer) GetType(ctx context.Context, req *troverpc.GetTypeRequest) (*troverpc.GetTypeResponse, error) {
	if s.catalog == nil {
		return nil, status.Error(codes.Unavailable, "type catalog is not configured")
	}
	if req == nil || req.GetUri() == "" {
		return nil, status.Error(codes.InvalidArgument, "uri is required")
	}
	entry, ok := s.catalog.Lookup(req.GetUri())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "type %q is not registered in catalog", req.GetUri())
	}
	var definition []byte
	if entry.Compiled != nil {
		definition = entry.Compiled.Definition.Definition
	}
	return &troverpc.GetTypeResponse{
		Summary:        typeSummaryFromEntry(entry),
		DefinitionJson: definition,
	}, nil
}

func (s *coreServicesServer) ExportType(ctx context.Context, req *troverpc.ExportTypeRequest) (*troverpc.ExportTypeResponse, error) {
	if s.catalog == nil {
		return nil, status.Error(codes.Unavailable, "type catalog is not configured")
	}
	if s.blobs == nil {
		return nil, status.Error(codes.Unavailable, "blob store is not configured")
	}
	if req == nil || req.GetUri() == "" {
		return nil, status.Error(codes.InvalidArgument, "uri is required")
	}
	entry, ok := s.catalog.Lookup(req.GetUri())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "type %q is not registered in catalog", req.GetUri())
	}
	data, err := s.catalog.Export(ctx, s.blobs, req.GetUri())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &troverpc.ExportTypeResponse{
		TtdJson:   data,
		SchemaRef: entry.SchemaRef,
	}, nil
}

func (s *coreServicesServer) ValidateTypeDefinition(ctx context.Context, req *troverpc.ValidateTypeDefinitionRequest) (*troverpc.ValidateTypeDefinitionResponse, error) {
	_ = ctx
	if req == nil || len(req.GetTtdJson()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ttd_json is required")
	}
	td, err := types.ValidateTypeDefinition(req.GetTtdJson())
	if err != nil {
		return &troverpc.ValidateTypeDefinitionResponse{
			Valid: false,
			Error: err.Error(),
		}, nil
	}
	return &troverpc.ValidateTypeDefinitionResponse{
		Valid: true,
		Uri:   td.ID,
	}, nil
}

func typeSummaryFromEntry(entry types.Entry) *troverpc.TypeSummary {
	summary := types.SummaryFromEntry(entry)
	return &troverpc.TypeSummary{
		Uri:         summary.URI,
		Title:       summary.Title,
		Description: summary.Description,
		Source:      summary.Source,
		SourcePath:  summary.SourcePath,
		SchemaRef:   summary.SchemaRef,
		Status:      summary.Status,
	}
}

func writeGRPCError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not allowed"),
		strings.Contains(msg, "is required"),
		strings.Contains(msg, "must be"),
		strings.Contains(msg, "not allowed for delete"),
		strings.Contains(msg, "payload"):
		return status.Error(codes.InvalidArgument, msg)
	default:
		return status.Errorf(codes.Internal, "%v", err)
	}
}

func queryGRPCError(err error) error {
	switch err {
	case query.ErrNotFound, query.ErrRecordNotFound:
		return status.Error(codes.NotFound, err.Error())
	case query.ErrEmptyQuery, query.ErrEmptyType, query.ErrEmptyRecordRef, query.ErrInvalidTimeRange:
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Errorf(codes.Internal, "%v", err)
	}
}

func parseOptionalProtoTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func queryEventToProto(e query.Event) *troverpc.Event {
	out := &troverpc.Event{
		Id:        e.ID,
		Time:      e.Time.Format(time.RFC3339),
		Type:      e.Type,
		Source:    e.Source,
		Payload:   e.Payload,
		Operation: e.Operation,
		RecordRef: e.RecordRef,
	}
	if len(e.Transforms) > 0 {
		out.Transforms = e.Transforms
	}
	if e.BlobRef != nil {
		out.BlobRef = *e.BlobRef
	}
	return out
}

func queryEventsToProto(events []query.Event) []*troverpc.Event {
	out := make([]*troverpc.Event, len(events))
	for i, e := range events {
		out[i] = queryEventToProto(e)
	}
	return out
}

func queryRecordsToProto(records []query.Record) []*troverpc.Record {
	out := make([]*troverpc.Record, len(records))
	for i, r := range records {
		out[i] = queryRecordToProto(r)
	}
	return out
}

func queryRecordToProto(r query.Record) *troverpc.Record {
	out := &troverpc.Record{
		RecordRef:    r.RecordRef,
		Version:      int32(r.Version), //nolint:gosec // G115: record version from projection
		Completeness: r.Completeness,
		Type:         r.Type,
		Source:       r.Source,
		Body:         r.Body,
		UpdatedAt:    r.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if r.ContentRef != nil {
		out.ContentRef = *r.ContentRef
	}
	return out
}

func summaryToProto(s query.Summary) *troverpc.Summary {
	byType := make(map[string]int32, len(s.ByType))
	for k, v := range s.ByType {
		byType[k] = int32(v) //nolint:gosec // G115: event counts from journal query
	}
	return &troverpc.Summary{
		TimeFrom: s.TimeFrom.Format(time.RFC3339),
		TimeTo:   s.TimeTo.Format(time.RFC3339),
		Total:    int32(s.Total), //nolint:gosec // G115: event count from journal query
		ByType:   byType,
		Notable:  queryEventsToProto(s.Notable),
	}
}
