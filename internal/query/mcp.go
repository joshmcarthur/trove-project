package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Serve starts the MCP query server on listen until ctx is cancelled.
func Serve(ctx context.Context, listen string, svc *Service) error {
	if svc == nil {
		return fmt.Errorf("query: service is required")
	}

	server := newMCPServer(svc)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	httpServer := &http.Server{
		Addr:    listen,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("trove: mcp listening on %s", listen)

	err := httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func newMCPServer(svc *Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "trove",
		Version: "0.1.0",
	}, nil)

	type getEventParams struct {
		ID string `json:"id" jsonschema:"ULID of the event to retrieve"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_event",
		Description: "Return a journal event by ULID",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params getEventParams) (*mcp.CallToolResult, any, error) {
		event, err := svc.GetEvent(ctx, params.ID)
		if err != nil {
			return nil, nil, err
		}
		data, err := json.Marshal(event)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(data)},
			},
		}, nil, nil
	})

	return server
}
