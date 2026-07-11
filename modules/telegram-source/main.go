package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-telegram/bot"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

type telegramSourceModule struct {
	ready atomic.Bool
}

func (m *telegramSourceModule) Run(ctx context.Context, core trovemodule.Core) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if core == nil {
		return fmt.Errorf("telegram-source: core connection is required")
	}

	m.ready.Store(true)
	defer m.ready.Store(false)

	return runTelegram(ctx, core, cfg)
}

func (m *telegramSourceModule) Healthcheck(context.Context) (*troverpc.HealthcheckResponse, error) {
	if m.ready.Load() {
		return &troverpc.HealthcheckResponse{Ok: true, Message: "telegram bot running"}, nil
	}
	return &troverpc.HealthcheckResponse{Ok: false, Message: "telegram bot not running"}, nil
}

func main() {
	trovemodule.Serve(&telegramSourceModule{})
}

func runTelegram(ctx context.Context, core trovemodule.Core, cfg config) error {
	svc := newBotService(cfg, core)

	opts := []bot.Option{
		bot.WithDefaultHandler(svc.handleUpdate),
		bot.WithHTTPClient(time.Duration(cfg.PollTimeoutSec)*time.Second, http.DefaultClient),
	}

	b, err := bot.New(cfg.BotToken, opts...)
	if err != nil {
		return fmt.Errorf("telegram-source: create bot: %w", err)
	}

	if err := svc.registerCommands(ctx, b); err != nil {
		log.Printf("telegram-source: set commands: %v", err)
	}

	b.Start(ctx)
	return nil
}
