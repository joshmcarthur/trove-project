package main

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func main() {
	trovemodule.Serve(trovemodule.RunFunc(func(ctx context.Context, core trovemodule.Core) error {
		_, err := trovemodule.EmitRecordFromEvent(ctx, core, &troverpc.Event{
			Type:    "trove://type/test/typed/emit/1",
			Source:  "typed-emit",
			Payload: []byte(`{"message":"hello"}`),
		})
		return err
	}))
}
