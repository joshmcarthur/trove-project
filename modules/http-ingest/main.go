package main

import (
	"context"

	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func main() {
	trovemodule.Serve(trovemodule.RunFunc(func(ctx context.Context, emit trovemodule.Emitter) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		return runHTTPServer(ctx, emit, cfg.Listen)
	}))
}
