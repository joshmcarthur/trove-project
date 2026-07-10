package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func runMQTT(ctx context.Context, emit trovemodule.Emitter, cfg config) error {
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(cfg.ClientID).
		SetAutoReconnect(false).
		SetConnectTimeout(10 * time.Second)

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("mqtt-source: connect to %s: timeout", cfg.Broker)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt-source: connect to %s: %w", cfg.Broker, err)
	}
	defer client.Disconnect(250)

	messageHandler := func(_ mqtt.Client, msg mqtt.Message) {
		event, err := buildEvent(msg.Topic(), msg.Payload())
		if err != nil {
			return
		}
		_ = emit.Emit(ctx, event)
	}

	for _, topic := range cfg.Topics {
		token := client.Subscribe(topic, cfg.QoS, messageHandler)
		if !token.WaitTimeout(10 * time.Second) {
			return fmt.Errorf("mqtt-source: subscribe %q: timeout", topic)
		}
		if err := token.Error(); err != nil {
			return fmt.Errorf("mqtt-source: subscribe %q: %w", topic, err)
		}
	}

	<-ctx.Done()
	return nil
}

func buildEvent(topic string, payload []byte) (*troverpc.Event, error) {
	body, err := buildPayload(topic, payload)
	if err != nil {
		return nil, err
	}

	return &troverpc.Event{
		Type:    topicToEventType(topic),
		Source:  topic,
		Payload: body,
	}, nil
}

func topicToEventType(topic string) string {
	slug := strings.ReplaceAll(topic, "/", ".")
	return "mqtt." + slug + ".received"
}

func buildPayload(topic string, payload []byte) ([]byte, error) {
	envelope := map[string]any{
		"metadata": map[string]string{"topic": topic},
	}

	if len(payload) == 0 {
		envelope["raw"] = ""
		return json.Marshal(envelope)
	}

	if json.Valid(payload) {
		var value any
		if err := json.Unmarshal(payload, &value); err != nil {
			return nil, err
		}
		envelope["message"] = value
		return json.Marshal(envelope)
	}

	envelope["raw"] = string(payload)
	return json.Marshal(envelope)
}
