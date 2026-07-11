package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type fieldConfig struct {
	Name     string `toml:"name"`
	Prompt   string `toml:"prompt"`
	Required bool   `toml:"required"`
}

type typeOption struct {
	Label      string `toml:"label"`
	TargetType string `toml:"target_type"`
}

type commandConfig struct {
	Command     string        `toml:"command"`
	Description string        `toml:"description"`
	TargetType  string        `toml:"target_type"`
	FastPath    bool          `toml:"fast_path"`
	Fields      []fieldConfig `toml:"fields"`
}

type config struct {
	BotToken       string          `toml:"bot_token"`
	BotTokenEnv    string          `toml:"bot_token_env"`
	AllowedChatIDs []int64         `toml:"allowed_chat_ids"`
	PollTimeoutSec int             `toml:"poll_timeout_sec"`
	MaxFileBytes   int64           `toml:"max_file_bytes"`
	SessionTTLMin  int             `toml:"session_ttl_min"`
	Types          []typeOption    `toml:"types"`
	Commands       []commandConfig `toml:"commands"`
}

func loadConfig() (config, error) {
	exe, err := os.Executable()
	if err != nil {
		return config{}, fmt.Errorf("telegram-source: executable path: %w", err)
	}
	return loadConfigFromDir(filepath.Dir(exe))
}

func loadConfigFromDir(dir string) (config, error) {
	manifestPath := filepath.Join(dir, "manifest.toml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return config{}, fmt.Errorf("telegram-source: read manifest: %w", err)
	}

	var raw struct {
		BotToken       string  `toml:"bot_token"`
		BotTokenEnv    string  `toml:"bot_token_env"`
		AllowedChatIDs []int64 `toml:"allowed_chat_ids"`
		PollTimeoutSec int     `toml:"poll_timeout_sec"`
		MaxFileBytes   int64   `toml:"max_file_bytes"`
		SessionTTLMin  int     `toml:"session_ttl_min"`
		Bot            struct {
			Types    []typeOption    `toml:"types"`
			Commands []commandConfig `toml:"commands"`
		} `toml:"bot"`
		Types    []typeOption    `toml:"types"`
		Commands []commandConfig `toml:"commands"`
	}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return config{}, fmt.Errorf("telegram-source: parse manifest: %w", err)
	}

	types := raw.Types
	if len(raw.Bot.Types) > 0 {
		types = raw.Bot.Types
	}
	commands := raw.Commands
	if len(raw.Bot.Commands) > 0 {
		commands = raw.Bot.Commands
	}

	cfg := config{
		BotToken:       strings.TrimSpace(raw.BotToken),
		BotTokenEnv:    strings.TrimSpace(raw.BotTokenEnv),
		AllowedChatIDs: raw.AllowedChatIDs,
		PollTimeoutSec: raw.PollTimeoutSec,
		MaxFileBytes:   raw.MaxFileBytes,
		SessionTTLMin:  raw.SessionTTLMin,
		Types:          types,
		Commands:       commands,
	}

	if cfg.BotToken == "" && cfg.BotTokenEnv != "" {
		cfg.BotToken = strings.TrimSpace(os.Getenv(cfg.BotTokenEnv))
	}
	if cfg.BotToken == "" {
		return config{}, fmt.Errorf("telegram-source: bot token is required (set bot_token or %s)", cfg.BotTokenEnv)
	}
	if len(cfg.AllowedChatIDs) == 0 {
		return config{}, fmt.Errorf("telegram-source: at least one allowed_chat_ids entry is required")
	}
	if cfg.PollTimeoutSec <= 0 {
		cfg.PollTimeoutSec = 30
	}
	if cfg.MaxFileBytes <= 0 {
		cfg.MaxFileBytes = 50 << 20
	}
	if cfg.SessionTTLMin <= 0 {
		cfg.SessionTTLMin = 30
	}
	if len(cfg.Types) == 0 {
		return config{}, fmt.Errorf("telegram-source: at least one [[bot.types]] entry is required")
	}
	for i, opt := range cfg.Types {
		if strings.TrimSpace(opt.Label) == "" || strings.TrimSpace(opt.TargetType) == "" {
			return config{}, fmt.Errorf("telegram-source: bot.types[%d]: label and target_type are required", i)
		}
	}
	for i, cmd := range cfg.Commands {
		if strings.TrimSpace(cmd.Command) == "" || strings.TrimSpace(cmd.TargetType) == "" {
			return config{}, fmt.Errorf("telegram-source: bot.commands[%d]: command and target_type are required", i)
		}
	}
	return cfg, nil
}

func (c config) allowedChat(chatID int64) bool {
	for _, id := range c.AllowedChatIDs {
		if id == chatID {
			return true
		}
	}
	return false
}

func (c config) fieldsForTarget(targetType string) []fieldConfig {
	for _, cmd := range c.Commands {
		if cmd.TargetType == targetType && len(cmd.Fields) > 0 {
			return cmd.Fields
		}
	}
	return nil
}

func (c config) commandByName(name string) (commandConfig, bool) {
	name = strings.TrimPrefix(strings.TrimSpace(name), "/")
	for _, cmd := range c.Commands {
		if cmd.Command == name {
			return cmd, true
		}
	}
	return commandConfig{}, false
}
