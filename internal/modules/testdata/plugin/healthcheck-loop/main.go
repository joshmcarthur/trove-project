package main

import (
	"context"
	"os"
	"strconv"
	"sync/atomic"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

type module struct {
	checks atomic.Int64
}

func (m *module) Run(ctx context.Context, _ trovemodule.Core) error {
	<-ctx.Done()
	return ctx.Err()
}

func (m *module) Healthcheck(context.Context) (*troverpc.HealthcheckResponse, error) {
	count := m.checks.Add(1)
	if path := os.Getenv("TROVE_TEST_HEALTHCHECK_FILE"); path != "" {
		_ = os.WriteFile(path, []byte(strconv.FormatInt(count, 10)), 0o644)
	}
	return &troverpc.HealthcheckResponse{Ok: true, Message: "alive"}, nil
}

func main() {
	trovemodule.Serve(&module{})
}
