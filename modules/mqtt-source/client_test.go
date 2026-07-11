package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTopicToEventType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		topic string
		want  string
	}{
		{"home/sensor/temp", "mqtt.home.sensor.temp.received"},
		{"devices/esphome/node1/state", "mqtt.devices.esphome.node1.state.received"},
		{"single", "mqtt.single.received"},
	}

	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			t.Parallel()
			if got := topicToEventType(tt.topic); got != tt.want {
				t.Errorf("topicToEventType(%q) = %q, want %q", tt.topic, got, tt.want)
			}
		})
	}
}

func TestBuildPayload(t *testing.T) {
	t.Parallel()

	const topic = "home/sensor/temp"

	tests := []struct {
		name    string
		payload []byte
		want    string
	}{
		{
			name:    "valid json object",
			payload: []byte(`{"v":21.5}`),
			want:    `{"message":{"v":21.5},"metadata":{"topic":"home/sensor/temp"}}`,
		},
		{
			name:    "valid json object preserves original content",
			payload: []byte(`{"metadata":{"device":"node-1"},"v":21.5}`),
			want:    `{"message":{"metadata":{"device":"node-1"},"v":21.5},"metadata":{"topic":"home/sensor/temp"}}`,
		},
		{
			name:    "valid json array",
			payload: []byte(`[1,2,3]`),
			want:    `{"message":[1,2,3],"metadata":{"topic":"home/sensor/temp"}}`,
		},
		{
			name:    "non-json bytes",
			payload: []byte("hello mqtt"),
			want:    `{"metadata":{"topic":"home/sensor/temp"},"raw":"hello mqtt"}`,
		},
		{
			name:    "empty payload",
			payload: []byte{},
			want:    `{"metadata":{"topic":"home/sensor/temp"},"raw":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildPayload(topic, tt.payload)
			if err != nil {
				t.Fatalf("buildPayload() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("buildPayload() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestBuildEvent(t *testing.T) {
	t.Parallel()

	event, err := buildEvent("home/sensor/temp", []byte(`{"v":21.5}`))
	if err != nil {
		t.Fatalf("buildEvent() error = %v", err)
	}
	if event.Type != "mqtt.home.sensor.temp.received" {
		t.Errorf("Type = %q, want mqtt.home.sensor.temp.received", event.Type)
	}
	if event.Source != "home/sensor/temp" {
		t.Errorf("Source = %q, want home/sensor/temp", event.Source)
	}
	if string(event.Payload) != `{"message":{"v":21.5},"metadata":{"topic":"home/sensor/temp"}}` {
		t.Errorf("Payload = %s, want message and metadata.topic", event.Payload)
	}
}

func TestLoadConfigFromDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := `name = "mqtt-source"
version = "1.0"
kind = "source"
provides = ["mqtt.message.received"]

broker = "tcp://broker.example:1883"
topics = ["home/#", "devices/+/state"]
qos = 1
username = "user"
password = "secret"
`
	if err := writeFile(t, dir, "manifest.toml", manifest); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfigFromDir(dir)
	if err != nil {
		t.Fatalf("loadConfigFromDir() error = %v", err)
	}
	if cfg.Broker != "tcp://broker.example:1883" {
		t.Errorf("Broker = %q", cfg.Broker)
	}
	if len(cfg.Topics) != 2 {
		t.Errorf("Topics = %v, want 2 entries", cfg.Topics)
	}
	if cfg.QoS != 1 {
		t.Errorf("QoS = %d, want 1", cfg.QoS)
	}
	if cfg.Username != "user" {
		t.Errorf("Username = %q", cfg.Username)
	}
}

func TestLoadConfigFromDirAppliesSettingsOverlay(t *testing.T) {
	dir := t.TempDir()
	manifest := `broker = "tcp://localhost:1883"
topics = ["home/#"]
`
	if err := writeFile(t, dir, "manifest.toml", manifest); err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.toml")
	if err := os.WriteFile(overlayPath, []byte(`broker = "tcp://override:1883"`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TROVE_MODULE_SETTINGS", overlayPath)

	cfg, err := loadConfigFromDir(dir)
	if err != nil {
		t.Fatalf("loadConfigFromDir() error = %v", err)
	}
	if cfg.Broker != "tcp://override:1883" {
		t.Fatalf("Broker = %q, want override", cfg.Broker)
	}
}

func TestLoadConfigFromDirValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest string
		wantErr  string
	}{
		{
			name: "missing broker",
			manifest: `topics = ["home/#"]
`,
			wantErr: "broker is required",
		},
		{
			name: "missing topics",
			manifest: `broker = "tcp://localhost:1883"
`,
			wantErr: "at least one topic is required",
		},
		{
			name: "invalid qos",
			manifest: `broker = "tcp://localhost:1883"
topics = ["home/#"]
qos = 3
`,
			wantErr: "qos must be 0, 1, or 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := writeFile(t, dir, "manifest.toml", tt.manifest); err != nil {
				t.Fatal(err)
			}
			_, err := loadConfigFromDir(dir)
			if err == nil {
				t.Fatal("loadConfigFromDir() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
