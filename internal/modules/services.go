package modules

import (
	"bytes"
	"context"
	"time"

	"github.com/joshmcarthur/trove/internal/blob"
	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type coreServicesServer struct {
	troverpc.UnimplementedCoreServicesServer
	journal journal.Journal
	policy  IngestPolicy
	blobs   blob.Store
	query   *query.Service
}

func (s *coreServicesServer) Emit(ctx context.Context, e *troverpc.Event) (*troverpc.EmitResponse, error) {
	if s.journal == nil {
		return nil, status.Error(codes.Unavailable, "journal is not configured")
	}
	event, err := rpcEventToJournal(e)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := s.policy.ValidateEvent(event); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := s.journal.Append(ctx, event); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &troverpc.EmitResponse{}, nil
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

func queryGRPCError(err error) error {
	switch err {
	case query.ErrNotFound:
		return status.Error(codes.NotFound, err.Error())
	case query.ErrEmptyQuery, query.ErrEmptyType, query.ErrInvalidTimeRange:
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
		Id:      e.ID,
		Time:    e.Time.Format(time.RFC3339),
		Type:    e.Type,
		Source:  e.Source,
		Payload: e.Payload,
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
