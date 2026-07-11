package main

import (
	"context"
	"sync/atomic"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

type mqttSourceModule struct {
	ready atomic.Bool
}

func (m *mqttSourceModule) Run(ctx context.Context, core trovemodule.Core) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	m.ready.Store(true)
	defer m.ready.Store(false)
	return runMQTT(ctx, core, cfg)
}

func (m *mqttSourceModule) Healthcheck(context.Context) (*troverpc.HealthcheckResponse, error) {
	if m.ready.Load() {
		return &troverpc.HealthcheckResponse{Ok: true, Message: "mqtt client running"}, nil
	}
	return &troverpc.HealthcheckResponse{Ok: false, Message: "mqtt client not running"}, nil
}

func main() {
	trovemodule.Serve(&mqttSourceModule{})
}
