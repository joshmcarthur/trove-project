package main

import (
	"encoding/json"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestBuildCaptureBodyText(t *testing.T) {
	t.Parallel()

	msg := &models.Message{
		ID:   42,
		Date: 1_700_000_000,
		Text: "hello world",
		Chat: models.Chat{ID: 99},
		From: &models.User{ID: 7, Username: "alice", FirstName: "Alice"},
	}

	body, err := buildCaptureBody(msg, "text", "hello world", "", nil)
	if err != nil {
		t.Fatalf("buildCaptureBody() error = %v", err)
	}

	var got captureBody
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.MessageKind != "text" || got.Text != "hello world" {
		t.Fatalf("got = %#v", got)
	}
	if got.ChatID != 99 || got.MessageID != 42 {
		t.Fatalf("ids = chat %d message %d", got.ChatID, got.MessageID)
	}
	if got.From == nil || got.From.Username != "alice" {
		t.Fatalf("from = %#v", got.From)
	}
}

func TestLargestPhotoFileID(t *testing.T) {
	t.Parallel()

	msg := &models.Message{
		Photo: []models.PhotoSize{
			{FileID: "small", FileSize: 100},
			{FileID: "large", FileSize: 5000},
			{FileID: "medium", FileSize: 1000},
		},
	}
	if got := largestPhotoFileID(msg); got != "large" {
		t.Fatalf("largestPhotoFileID() = %q, want large", got)
	}
}

func TestFieldDefaultsFromDraft(t *testing.T) {
	t.Parallel()

	got := fieldDefaultsFromDraft(&captureDraft{Text: "caption"})
	if got["text"] != "caption" {
		t.Fatalf("fieldDefaultsFromDraft() = %#v", got)
	}
}

func TestMergeFieldPayload(t *testing.T) {
	t.Parallel()

	out, err := mergeFieldPayload(map[string]string{"title": "Hi", "url": "https://example.com"})
	if err != nil {
		t.Fatalf("mergeFieldPayload() error = %v", err)
	}
	var payload map[string]string
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["title"] != "Hi" || payload["url"] != "https://example.com" {
		t.Fatalf("payload = %#v", payload)
	}
}
