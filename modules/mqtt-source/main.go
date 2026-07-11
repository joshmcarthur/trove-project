package main

import (
	"context"
	"sync/atomic"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

type mqttSourceModule struct {
	ready atomic.Bool
	state *subscriptionState
}

func (m *mqttSourceModule) Run(ctx context.Context, core trovemodule.Core) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	m.state = newSubscriptionState(cfg.Topics)
	m.ready.Store(true)
	defer m.ready.Store(false)
	return runMQTT(ctx, core, cfg, m.state)
}

func (m *mqttSourceModule) Healthcheck(context.Context) (*troverpc.HealthcheckResponse, error) {
	if !m.ready.Load() || m.state == nil {
		return &troverpc.HealthcheckResponse{Ok: false, Message: "mqtt client not running"}, nil
	}
	ok, message := m.state.healthMessage()
	return &troverpc.HealthcheckResponse{Ok: ok, Message: message}, nil
}

func main() {
	trovemodule.Serve(&mqttSourceModule{})
}
