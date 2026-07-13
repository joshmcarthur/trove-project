package main

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func main() {
	trovemodule.Serve(trovemodule.RunFunc(func(ctx context.Context, core trovemodule.Core) error {
		_, err := trovemodule.AppendRevisionFromMessage(ctx, core, &troverpc.Revision{
			Type:    "test.emit.once",
			Source:  "emit-once",
			Payload: []byte(`{"hello":"world"}`),
		})
		return err
	}))
}
