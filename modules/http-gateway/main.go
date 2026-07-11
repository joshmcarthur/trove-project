package main

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"sync/atomic"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

const bearerValidatorID = "bearer"

type httpGatewayModule struct {
	ready atomic.Bool
	cfg   config
}

func (m *httpGatewayModule) Run(ctx context.Context, _ trovemodule.Core) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	m.cfg = cfg
	m.ready.Store(true)
	defer m.ready.Store(false)
	<-ctx.Done()
	return nil
}

func (m *httpGatewayModule) ValidateAuth(_ context.Context, req *troverpc.AuthRequest) (*troverpc.AuthResponse, error) {
	if !m.ready.Load() {
		return denied(http.StatusServiceUnavailable, "service unavailable"), nil
	}
	if req.GetValidatorId() != bearerValidatorID {
		return denied(http.StatusBadRequest, "unknown validator"), nil
	}
	if m.cfg.Token == "" {
		return denied(http.StatusServiceUnavailable, "bearer token not configured"), nil
	}
	if authorized(req.GetHeaders(), m.cfg.Token) {
		return &troverpc.AuthResponse{Allowed: true}, nil
	}
	return denied(http.StatusUnauthorized, "unauthorized"), nil
}

func (m *httpGatewayModule) Healthcheck(context.Context) (*troverpc.HealthcheckResponse, error) {
	if m.ready.Load() {
		return &troverpc.HealthcheckResponse{Ok: true, Message: "auth validators ready"}, nil
	}
	return &troverpc.HealthcheckResponse{Ok: false, Message: "auth validators not ready"}, nil
}

func authorized(headers map[string]string, token string) bool {
	const prefix = "Bearer "
	auth := headerValue(headers, "Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	got := strings.TrimPrefix(auth, prefix)
	if len(got) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

func headerValue(headers map[string]string, key string) string {
	if headers == nil {
		return ""
	}
	if value, ok := headers[key]; ok {
		return value
	}
	for k, value := range headers {
		if strings.EqualFold(k, key) {
			return value
		}
	}
	return ""
}

func denied(status int32, message string) *troverpc.AuthResponse {
	return &troverpc.AuthResponse{
		Allowed: false,
		Status:  status,
		Message: message,
	}
}

func main() {
	trovemodule.Serve(&httpGatewayModule{})
}
