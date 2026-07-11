package main

import (
	"context"
	"testing"
)

func TestMQTTSourceHealthcheck(t *testing.T) {
	mod := &mqttSourceModule{}
	resp, err := mod.Healthcheck(context.Background())
	if err != nil {
		t.Fatalf("Healthcheck() error = %v", err)
	}
	if resp.Ok {
		t.Fatalf("Healthcheck() ok = true before Run, want false")
	}

	mod.state = newSubscriptionState([]string{"home/#"})
	mod.ready.Store(true)
	mod.state.setConnected(true)
	mod.state.setSubscribed("home/#", true)

	resp, err = mod.Healthcheck(context.Background())
	if err != nil {
		t.Fatalf("Healthcheck() error = %v", err)
	}
	if !resp.Ok {
		t.Fatalf("Healthcheck() ok = false, msg = %q", resp.Message)
	}
	if resp.Message == "" {
		t.Fatal("Healthcheck() message is empty")
	}
}
