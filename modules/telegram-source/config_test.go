package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFromDirNestedBotTables(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := `
bot_token = "test-token"
allowed_chat_ids = [12345]

[[bot.types]]
label = "Quick note"
target_type = "trove://type/note/quick/1"

[[bot.commands]]
command = "note"
description = "Quick note"
target_type = "trove://type/note/quick/1"
fast_path = true

  [[bot.commands.fields]]
  name = "title"
  prompt = "Title?"
  required = false
`
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cfg, err := loadConfigFromDir(dir)
	if err != nil {
		t.Fatalf("loadConfigFromDir() error = %v", err)
	}
	if cfg.BotToken != "test-token" {
		t.Fatalf("BotToken = %q", cfg.BotToken)
	}
	if len(cfg.Types) != 1 || cfg.Types[0].TargetType != "trove://type/note/quick/1" {
		t.Fatalf("Types = %#v", cfg.Types)
	}
	if len(cfg.Commands) != 1 || cfg.Commands[0].Command != "note" {
		t.Fatalf("Commands = %#v", cfg.Commands)
	}
	if len(cfg.Commands[0].Fields) != 1 || cfg.Commands[0].Fields[0].Name != "title" {
		t.Fatalf("Fields = %#v", cfg.Commands[0].Fields)
	}
}

func TestLoadConfigRequiresTokenAndChats(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := `
bot_token_env = "MISSING_TOKEN"
allowed_chat_ids = []

[[bot.types]]
label = "Quick note"
target_type = "trove://type/note/quick/1"
`
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if _, err := loadConfigFromDir(dir); err == nil {
		t.Fatal("loadConfigFromDir() error = nil, want error")
	}
}

func TestConfigAllowedChat(t *testing.T) {
	t.Parallel()

	cfg := config{AllowedChatIDs: []int64{100, 200}}
	if !cfg.allowedChat(100) {
		t.Fatal("allowedChat(100) = false, want true")
	}
	if cfg.allowedChat(300) {
		t.Fatal("allowedChat(300) = true, want false")
	}
}

func TestLoadConfigFromDirAppliesSettingsOverlay(t *testing.T) {
	dir := t.TempDir()
	manifest := `
bot_token = "test-token"
allowed_chat_ids = [1]

[[bot.types]]
label = "Quick note"
target_type = "trove://type/note/quick/1"
`
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	overlayPath := filepath.Join(dir, "overlay.toml")
	if err := os.WriteFile(overlayPath, []byte("allowed_chat_ids = [99]\n"), 0o600); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	t.Setenv("TROVE_MODULE_SETTINGS", overlayPath)

	cfg, err := loadConfigFromDir(dir)
	if err != nil {
		t.Fatalf("loadConfigFromDir() error = %v", err)
	}
	if len(cfg.AllowedChatIDs) != 1 || cfg.AllowedChatIDs[0] != 99 {
		t.Fatalf("AllowedChatIDs = %#v, want [99]", cfg.AllowedChatIDs)
	}
}

func TestFieldsForTarget(t *testing.T) {
	t.Parallel()

	cfg := config{
		Commands: []commandConfig{{
			TargetType: "trove://type/note/bookmark/1",
			Fields: []fieldConfig{{
				Name:     "url",
				Prompt:   "URL?",
				Required: true,
			}},
		}},
	}
	fields := cfg.fieldsForTarget("trove://type/note/bookmark/1")
	if len(fields) != 1 || fields[0].Name != "url" {
		t.Fatalf("fieldsForTarget() = %#v", fields)
	}
}
