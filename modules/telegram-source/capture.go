package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-telegram/bot/models"
)

type telegramUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
}

type telegramFileMeta struct {
	FileID   string `json:"file_id,omitempty"`
	FileName string `json:"file_name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

type captureBody struct {
	Time        string            `json:"time,omitempty"`
	BlobRef     string            `json:"blob_ref,omitempty"`
	Text        string            `json:"text,omitempty"`
	MessageID   int               `json:"message_id"`
	ChatID      int64             `json:"chat_id"`
	MessageKind string            `json:"message_kind"`
	From        *telegramUser     `json:"from,omitempty"`
	File        *telegramFileMeta `json:"file,omitempty"`
}

func messageTime(msg *models.Message) string {
	if msg == nil {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return time.Unix(int64(msg.Date), 0).UTC().Format(time.RFC3339)
}

func userFromMessage(msg *models.Message) *telegramUser {
	if msg == nil || msg.From == nil {
		return nil
	}
	return &telegramUser{
		ID:        msg.From.ID,
		Username:  msg.From.Username,
		FirstName: msg.From.FirstName,
	}
}

func buildCaptureBody(msg *models.Message, kind string, text string, blobRef string, file *telegramFileMeta) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("telegram-source: message is required")
	}
	body := captureBody{
		Time:        messageTime(msg),
		BlobRef:     blobRef,
		Text:        text,
		MessageID:   msg.ID,
		ChatID:      msg.Chat.ID,
		MessageKind: kind,
		From:        userFromMessage(msg),
		File:        file,
	}
	out, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("telegram-source: marshal capture body: %w", err)
	}
	return out, nil
}

func extractMessageText(msg *models.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Text != "" {
		return msg.Text
	}
	return msg.Caption
}

func largestPhotoFileID(msg *models.Message) string {
	if msg == nil || len(msg.Photo) == 0 {
		return ""
	}
	best := msg.Photo[0]
	for _, p := range msg.Photo[1:] {
		if p.FileSize > best.FileSize {
			best = p
		}
	}
	return best.FileID
}

func fieldDefaultsFromDraft(draft *captureDraft) map[string]string {
	out := make(map[string]string)
	if draft == nil {
		return out
	}
	if draft.Text != "" {
		out["text"] = draft.Text
	}
	return out
}

func mergeFieldPayload(collected map[string]string) ([]byte, error) {
	if len(collected) == 0 {
		return nil, nil
	}
	out, err := json.Marshal(collected)
	if err != nil {
		return nil, err
	}
	return out, nil
}
