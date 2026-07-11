package main

import (
	"strings"
	"testing"
)

func TestSubscriptionStateHealthMessage(t *testing.T) {
	t.Parallel()

	state := newSubscriptionState([]string{"home/#", "devices/+/state"})

	ok, msg := state.healthMessage()
	if ok {
		t.Fatalf("healthMessage() ok = true, want false when disconnected")
	}
	if msg != "mqtt client disconnected" {
		t.Fatalf("healthMessage() = %q", msg)
	}

	state.setConnected(true)
	state.setSubscribed("home/#", true)
	state.setSubscribed("devices/+/state", false)

	ok, msg = state.healthMessage()
	if ok {
		t.Fatalf("healthMessage() ok = true, want false with pending subscription")
	}
	if !strings.Contains(msg, "home/#=subscribed") {
		t.Fatalf("healthMessage() = %q, want home/# subscribed", msg)
	}
	if !strings.Contains(msg, "devices/+/state=pending") {
		t.Fatalf("healthMessage() = %q, want devices pending", msg)
	}

	state.setSubscribed("devices/+/state", true)
	ok, msg = state.healthMessage()
	if !ok {
		t.Fatalf("healthMessage() ok = false, want true: %q", msg)
	}
	if !strings.HasPrefix(msg, "connected;") {
		t.Fatalf("healthMessage() = %q, want connected prefix", msg)
	}
}

func TestSubscriptionStateDisconnectClearsSubscriptions(t *testing.T) {
	t.Parallel()

	state := newSubscriptionState([]string{"home/#"})
	state.setConnected(true)
	state.setSubscribed("home/#", true)

	state.setConnected(false)
	ok, msg := state.healthMessage()
	if ok || msg != "mqtt client disconnected" {
		t.Fatalf("healthMessage() = (%v, %q), want disconnected", ok, msg)
	}

	state.setConnected(true)
	ok, msg = state.healthMessage()
	if ok {
		t.Fatalf("healthMessage() ok = true after reconnect before subscribe, msg = %q", msg)
	}
}
