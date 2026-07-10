package main

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func main() {
	trovemodule.Serve(trovemodule.RunFunc(func(ctx context.Context, emit trovemodule.Emitter) error {
		return emit.Emit(ctx, &troverpc.Event{
			Type:    "test.emit.once",
			Source:  "emit-once",
			Payload: []byte(`{"hello":"world"}`),
		})
	}))
}
