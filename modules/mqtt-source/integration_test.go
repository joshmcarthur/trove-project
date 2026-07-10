package main

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

type channelEmitter struct {
	mu     sync.Mutex
	events []*troverpc.Event
	notify chan struct{}
}

func newChannelEmitter() *channelEmitter {
	return &channelEmitter{notify: make(chan struct{}, 8)}
}

func (e *channelEmitter) Emit(_ context.Context, event *troverpc.Event) error {
	e.mu.Lock()
	e.events = append(e.events, event)
	e.mu.Unlock()
	select {
	case e.notify <- struct{}{}:
	default:
	}
	return nil
}

func (e *channelEmitter) waitForEvents(t *testing.T, count int, timeout time.Duration) []*troverpc.Event {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		e.mu.Lock()
		n := len(e.events)
		e.mu.Unlock()
		if n >= count {
			e.mu.Lock()
			out := append([]*troverpc.Event(nil), e.events...)
			e.mu.Unlock()
			return out
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("timed out waiting for %d events, got %d", count, n)
		}
		select {
		case <-e.notify:
		case <-time.After(remaining):
			e.mu.Lock()
			n = len(e.events)
			e.mu.Unlock()
			t.Fatalf("timed out waiting for %d events, got %d", count, n)
		}
	}
}

func startTestBroker(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	server := mqtt.New(nil)
	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
		t.Fatalf("AddHook: %v", err)
	}

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "test",
		Address: addr,
	})
	if err := server.AddListener(tcp); err != nil {
		t.Fatalf("AddListener: %v", err)
	}

	go func() {
		_ = server.Serve()
	}()
	t.Cleanup(func() { _ = server.Close() })

	return "tcp://" + addr
}

func TestRunMQTTSubscribeAndEmit(t *testing.T) {
	broker := startTestBroker(t)
	emit := newChannelEmitter()

	cfg := config{
		Broker:   broker,
		ClientID: "test-client",
		Topics:   []string{"home/#"},
		QoS:      0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- runMQTT(ctx, emit, cfg)
	}()

	publisher := pahomqtt.NewClient(pahomqtt.NewClientOptions().AddBroker(broker).SetClientID("publisher"))
	token := publisher.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatal("publisher connect timeout")
	}
	if err := token.Error(); err != nil {
		t.Fatalf("publisher connect: %v", err)
	}
	defer publisher.Disconnect(250)

	time.Sleep(100 * time.Millisecond)

	pubToken := publisher.Publish("home/sensor/temp", 0, false, []byte(`{"v":21.5}`))
	if !pubToken.WaitTimeout(5 * time.Second) {
		t.Fatal("publish timeout")
	}
	if err := pubToken.Error(); err != nil {
		t.Fatalf("publish: %v", err)
	}

	events := emit.waitForEvents(t, 1, 5*time.Second)
	if events[0].Type != "mqtt.home.sensor.temp.received" {
		t.Errorf("Type = %q, want mqtt.home.sensor.temp.received", events[0].Type)
	}
	if events[0].Source != "home/sensor/temp" {
		t.Errorf("Source = %q, want home/sensor/temp", events[0].Source)
	}
	if string(events[0].Payload) != `{"message":{"v":21.5},"metadata":{"topic":"home/sensor/temp"}}` {
		t.Errorf("Payload = %s, want message and metadata.topic", events[0].Payload)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("runMQTT() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runMQTT did not exit after cancel")
	}
}

var _ trovemodule.Emitter = (*channelEmitter)(nil)
