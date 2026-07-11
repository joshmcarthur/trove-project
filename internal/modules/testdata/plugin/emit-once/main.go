package main

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func main() {
	trovemodule.Serve(trovemodule.CoreRunFunc(func(ctx context.Context, core trovemodule.Core) error {
		return core.Emit(ctx, &troverpc.Event{
			Type:    "test.emit.once",
			Source:  "emit-once",
			Payload: []byte(`{"hello":"world"}`),
		})
	}))
}
