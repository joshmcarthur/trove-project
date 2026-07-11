package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/joshmcarthur/trove/internal/modules"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// BuiltinRoute is an in-process HTTP handler registered on the gateway mux.
type BuiltinRoute struct {
	Method  string
	Pattern string
	Handler http.Handler
}

// Config holds gateway listener settings.
type Config struct {
	Listen        string
	MaxBodyBytes  int64
	AuthValidator string
}

// Gateway routes HTTP requests to module HandleHTTP RPC clients.
type Gateway struct {
	listen        string
	maxBodyBytes  int64
	authValidator string
	routes        []modules.HTTPRouteEntry
	registry      *modules.HTTPRegistry
	authRegistry  *modules.AuthRegistry
	builtins      []BuiltinRoute
}

// New constructs a Gateway. builtins may be nil.
func New(cfg Config, routes []modules.HTTPRouteEntry, registry *modules.HTTPRegistry, authRegistry *modules.AuthRegistry, builtins []BuiltinRoute) (*Gateway, error) {
	if cfg.Listen == "" {
		return nil, fmt.Errorf("gateway: listen is required")
	}
	if registry == nil {
		return nil, fmt.Errorf("gateway: registry is required")
	}
	maxBody := cfg.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = 10 << 20
	}
	return &Gateway{
		listen:        cfg.Listen,
		maxBodyBytes:  maxBody,
		authValidator: cfg.AuthValidator,
		routes:        routes,
		registry:      registry,
		authRegistry:  authRegistry,
		builtins:      builtins,
	}, nil
}

// Serve listens until ctx is cancelled.
func (g *Gateway) Serve(ctx context.Context) error {
	mux := http.NewServeMux()

	for _, builtin := range g.builtins {
		method := strings.ToUpper(builtin.Method)
		pattern := builtin.Pattern
		handler := builtin.Handler
		mux.HandleFunc(method+" "+pattern, func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
		})
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != method {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		})
	}

	mux.HandleFunc("/", g.handle)

	srv := &http.Server{
		Addr:    g.listen,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("trove: http gateway listening on %s", g.listen)

	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (g *Gateway) handle(w http.ResponseWriter, r *http.Request) {
	route, pathValues, ok := g.matchRoute(r.Method, r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	headers := copyHeaders(r.Header)
	if err := g.authorize(w, r, route, pathValues, headers); err != nil {
		return
	}

	maxBody := g.maxBodyBytes
	if route.Route.MaxBodyBytes > 0 {
		maxBody = route.Route.MaxBodyBytes
	}

	body, err := readBody(w, r, maxBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req := &troverpc.HTTPRequest{
		Method:         r.Method,
		Path:           r.URL.Path,
		MatchedPattern: route.Route.Path,
		PathValues:     pathValues,
		Headers:        headers,
		Body:           body,
	}

	client, ok := g.registry.Get(route.Module)
	if !ok {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	resp, err := client.HandleHTTP(r.Context(), req)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	writeHTTPResponse(w, resp)
}

func (g *Gateway) authorize(w http.ResponseWriter, r *http.Request, route modules.HTTPRouteEntry, pathValues map[string]string, headers map[string]string) error {
	validatorRef, skip, err := modules.ResolveRouteAuth(route.Route.Auth, g.authValidator)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return err
	}
	if skip {
		return nil
	}
	if g.authRegistry == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return fmt.Errorf("gateway: auth registry is required")
	}

	moduleName, validatorID, err := modules.ParseAuthValidatorRef(validatorRef)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return err
	}

	client, ok := g.authRegistry.Get(validatorRef)
	if !ok {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return fmt.Errorf("gateway: auth validator %q unavailable", validatorRef)
	}

	resp, err := client.ValidateAuth(r.Context(), &troverpc.AuthRequest{
		ValidatorId:    validatorID,
		Method:         r.Method,
		Path:           r.URL.Path,
		MatchedPattern: route.Route.Path,
		PathValues:     pathValues,
		Headers:        copyHeaders(r.Header),
		RouteModule:    route.Module,
	})
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return err
	}
	if resp == nil || !resp.Allowed {
		status := http.StatusUnauthorized
		if resp != nil && resp.Status > 0 {
			status = int(resp.Status)
		}
		message := "unauthorized"
		if resp != nil && resp.Message != "" {
			message = resp.Message
		}
		http.Error(w, message, status)
		return fmt.Errorf("gateway: unauthorized by %s.%s", moduleName, validatorID)
	}
	for key, value := range resp.Headers {
		headers[key] = value
	}
	return nil
}

func (g *Gateway) matchRoute(method, urlPath string) (modules.HTTPRouteEntry, map[string]string, bool) {
	method = strings.ToUpper(method)
	for _, route := range g.routes {
		if strings.ToUpper(route.Route.Method) != method {
			continue
		}
		if values, ok := matchPattern(route.Route.Path, urlPath); ok {
			return route, values, true
		}
	}
	return modules.HTTPRouteEntry{}, nil, false
}

func matchPattern(pattern, urlPath string) (map[string]string, bool) {
	patParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(urlPath, "/"), "/")
	if len(patParts) != len(pathParts) {
		return nil, false
	}
	values := make(map[string]string)
	for i, part := range patParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			if name == "" {
				return nil, false
			}
			values[name] = pathParts[i]
			continue
		}
		if part != pathParts[i] {
			return nil, false
		}
	}
	return values, true
}

func readBody(w http.ResponseWriter, r *http.Request, maxBodyBytes int64) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	limited := http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer limited.Close()
	body, err := io.ReadAll(limited)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, fmt.Errorf("request body too large")
		}
		return nil, fmt.Errorf("read body")
	}
	return body, nil
}

func copyHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for key, values := range h {
		if len(values) > 0 {
			out[key] = values[0]
		}
	}
	return out
}

func writeHTTPResponse(w http.ResponseWriter, resp *troverpc.HTTPResponse) {
	if resp == nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	for key, value := range resp.Headers {
		w.Header().Set(key, value)
	}
	status := int(resp.Status)
	if status == 0 {
		status = http.StatusOK
	}
	if len(resp.Body) == 0 {
		w.WriteHeader(status)
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(resp.Body)
}

// ValidateRoutes ensures no builtin route conflicts with module routes.
func ValidateRoutes(routes []modules.HTTPRouteEntry, builtins []BuiltinRoute) error {
	seen := make(map[string]struct{})
	for _, route := range routes {
		key := strings.ToUpper(route.Route.Method) + " " + route.Route.Path
		if _, exists := seen[key]; exists {
			return fmt.Errorf("gateway: duplicate route %s", key)
		}
		seen[key] = struct{}{}
	}
	for _, builtin := range builtins {
		key := strings.ToUpper(builtin.Method) + " " + builtin.Pattern
		if _, exists := seen[key]; exists {
			return fmt.Errorf("gateway: duplicate route %s", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}
