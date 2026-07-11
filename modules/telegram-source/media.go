package main

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func downloadTelegramFile(ctx context.Context, b *bot.Bot, fileID string, maxBytes int64) ([]byte, error) {
	if fileID == "" {
		return nil, fmt.Errorf("telegram-source: file id is required")
	}
	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: fileID})
	if err != nil {
		return nil, fmt.Errorf("telegram-source: get file: %w", err)
	}
	if file.FileSize > maxBytes {
		return nil, fmt.Errorf("telegram-source: file exceeds max_file_bytes (%d)", maxBytes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.FileDownloadLink(file), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram-source: download file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram-source: download file: status %s", resp.Status)
	}

	limited := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("telegram-source: file exceeds max_file_bytes (%d)", maxBytes)
	}
	return data, nil
}

func fileMetaFromDocument(doc *models.Document) *telegramFileMeta {
	if doc == nil {
		return nil
	}
	return &telegramFileMeta{
		FileID:   doc.FileID,
		FileName: doc.FileName,
		MimeType: doc.MimeType,
		Size:     doc.FileSize,
	}
}

func fileMetaFromVoice(voice *models.Voice) *telegramFileMeta {
	if voice == nil {
		return nil
	}
	return &telegramFileMeta{
		FileID:   voice.FileID,
		MimeType: voice.MimeType,
		Size:     voice.FileSize,
	}
}

func fileMetaFromPhoto(fileID string, size int64) *telegramFileMeta {
	return &telegramFileMeta{
		FileID:   fileID,
		MimeType: "image/jpeg",
		Size:     size,
	}
}
