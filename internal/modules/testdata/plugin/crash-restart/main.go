package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func main() {
	trovemodule.Serve(trovemodule.RunFunc(func(ctx context.Context, core trovemodule.Core) error {
		counterPath := os.Getenv("TROVE_TEST_COUNTER_FILE")
		var count int
		if counterPath != "" {
			if data, err := os.ReadFile(counterPath); err == nil {
				count, _ = strconv.Atoi(string(data))
			}
			count++
			_ = os.WriteFile(counterPath, []byte(strconv.Itoa(count)), 0o644)
		}

		if _, err := trovemodule.EmitRecordFromEvent(ctx, core, &troverpc.Event{
			Type:    "test.crash.restart",
			Source:  "crash-restart",
			Payload: []byte(fmt.Sprintf(`{"run":%d}`, count)),
		}); err != nil {
			return err
		}

		return fmt.Errorf("simulated crash")
	}))
}
