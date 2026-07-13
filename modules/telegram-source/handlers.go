package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/joshmcarthur/trove/pkg/classify"
	"github.com/joshmcarthur/trove/pkg/trovemodule"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

const callbackTypePrefix = "t:"

type botService struct {
	cfg      config
	core     trovemodule.Core
	store    *captureStore
	sessions *sessionStore
}

func newBotService(cfg config, core trovemodule.Core) *botService {
	return &botService{
		cfg:      cfg,
		core:     core,
		store:    &captureStore{core: core},
		sessions: newSessionStore(cfg.SessionTTLMin),
	}
}

func (s *botService) registerCommands(ctx context.Context, b *bot.Bot) error {
	commands := []models.BotCommand{
		{Command: "cancel", Description: "Cancel the current capture"},
		{Command: "classify", Description: "Classify by ID: /classify <record_ref> <type>"},
	}
	for _, cmd := range s.cfg.Commands {
		if !cmd.FastPath {
			continue
		}
		commands = append(commands, models.BotCommand{
			Command:     cmd.Command,
			Description: cmd.Description,
		})
	}
	_, err := b.SetMyCommands(ctx, &bot.SetMyCommandsParams{Commands: commands})
	return err
}

func (s *botService) handleUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil {
		return
	}
	if update.CallbackQuery != nil {
		s.handleCallback(ctx, b, update)
		return
	}
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !s.cfg.allowedChat(chatID) {
		return
	}
	if update.Message.Text != "" && strings.HasPrefix(update.Message.Text, "/") {
		s.handleCommand(ctx, b, update.Message)
		return
	}
	s.handleContent(ctx, b, update.Message)
}

func (s *botService) handleCommand(ctx context.Context, b *bot.Bot, msg *models.Message) {
	text := strings.TrimSpace(msg.Text)
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return
	}
	cmd := strings.TrimPrefix(parts[0], "/")
	cmd = strings.Split(cmd, "@")[0]

	switch cmd {
	case "cancel":
		s.sessions.clear(msg.Chat.ID)
		s.sendText(ctx, b, msg.Chat.ID, "Cancelled.")
		return
	case "skip":
		s.handleSkip(ctx, b, msg)
		return
	case "classify":
		s.handleClassifyCommand(ctx, b, msg, parts[1:])
		return
	}

	if cmdCfg, ok := s.cfg.commandByName(cmd); ok && cmdCfg.FastPath {
		s.startFastPath(ctx, b, msg, cmdCfg)
		return
	}

	s.sendText(ctx, b, msg.Chat.ID, "Unknown command. Send content to capture, or use /cancel.")
}

func (s *botService) handleContent(ctx context.Context, b *bot.Bot, msg *models.Message) {
	chatID := msg.Chat.ID
	if _, busy := s.sessions.activePendingID(chatID); busy {
		if id, ok := s.sessions.get(chatID); ok && id != nil && id.PendingRecordRef != "" {
			s.sendText(ctx, b, chatID, fmt.Sprintf("Finish classifying %s or /cancel first.", id.PendingRecordRef))
			return
		}
		s.sendText(ctx, b, chatID, "Finish the current command or /cancel first.")
		return
	}

	if sess, ok := s.sessions.get(chatID); ok && sess != nil {
		switch sess.Mode {
		case modeFastPath:
			if sess.AwaitingContent {
				s.handleFastPathContent(ctx, b, msg, sess)
				return
			}
			if sess.FieldIndex >= 0 {
				s.handleFieldAnswer(ctx, b, msg, sess)
				return
			}
		case modeClassify:
			if sess.FieldIndex >= 0 {
				s.handleFieldAnswer(ctx, b, msg, sess)
				return
			}
		}
	}

	s.handleCapture(ctx, b, msg)
}

func (s *botService) handleCapture(ctx context.Context, b *bot.Bot, msg *models.Message) {
	draft, err := s.buildDraft(ctx, b, msg)
	if err != nil {
		log.Printf("telegram-source: capture chat %d: %v", msg.Chat.ID, err)
		s.sendText(ctx, b, msg.Chat.ID, "Could not capture that message.")
		return
	}

	result, err := classify.Capture(ctx, s.store, "telegram", draft.CaptureJSON)
	if err != nil {
		log.Printf("telegram-source: pending chat %d: %v", msg.Chat.ID, err)
		s.sendText(ctx, b, msg.Chat.ID, "Could not save capture.")
		return
	}

	s.sessions.set(msg.Chat.ID, &session{
		Mode:             modeClassify,
		PendingRecordRef: result.RecordRef,
		FieldIndex:       -1,
		Collected:        fieldDefaultsFromDraft(draft),
		Draft:            draft,
	})

	text := fmt.Sprintf("Captured %s\nWhat is this?", result.RecordRef)
	s.sendTextWithKeyboard(ctx, b, msg.Chat.ID, text, s.typeKeyboard())
}

func (s *botService) handleCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil || cq.Message.Message == nil {
		return
	}
	chatID := cq.Message.Message.Chat.ID
	if !s.cfg.allowedChat(chatID) {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
	})

	data := cq.Data
	if !strings.HasPrefix(data, callbackTypePrefix) {
		return
	}
	idxStr := strings.TrimPrefix(data, callbackTypePrefix)
	var idx int
	if _, err := fmt.Sscanf(idxStr, "%d", &idx); err != nil || idx < 0 || idx >= len(s.cfg.Types) {
		return
	}

	sess, ok := s.sessions.get(chatID)
	if !ok || sess == nil || sess.PendingRecordRef == "" {
		s.sendText(ctx, b, chatID, "No active capture. Send content to start.")
		return
	}

	targetType := s.cfg.Types[idx].TargetType
	s.beginClassifyFields(ctx, b, chatID, sess, targetType)
}

func (s *botService) beginClassifyFields(ctx context.Context, b *bot.Bot, chatID int64, sess *session, targetType string) {
	sess.TargetType = targetType
	fields := s.cfg.fieldsForTarget(targetType)
	sess.FieldIndex = 0
	s.sessions.set(chatID, sess)

	if len(fields) == 0 {
		s.finishClassify(ctx, b, chatID, sess)
		return
	}
	s.promptNextField(ctx, b, chatID, sess, fields)
}

func (s *botService) handleFieldAnswer(ctx context.Context, b *bot.Bot, msg *models.Message, sess *session) {
	fields := s.cfg.fieldsForTarget(sess.TargetType)
	if sess.FieldIndex < 0 || sess.FieldIndex >= len(fields) {
		return
	}
	field := fields[sess.FieldIndex]
	value := strings.TrimSpace(msg.Text)
	if value == "" && field.Required {
		s.sendText(ctx, b, msg.Chat.ID, "A value is required.")
		return
	}
	if sess.Collected == nil {
		sess.Collected = make(map[string]string)
	}
	if value != "" {
		sess.Collected[field.Name] = value
	}
	sess.FieldIndex++
	s.sessions.set(msg.Chat.ID, sess)

	if sess.FieldIndex >= len(fields) {
		if sess.Mode == modeClassify {
			s.finishClassify(ctx, b, msg.Chat.ID, sess)
		} else {
			s.finishFastPath(ctx, b, msg.Chat.ID, sess)
		}
		return
	}
	s.promptNextField(ctx, b, msg.Chat.ID, sess, fields)
}

func (s *botService) handleSkip(ctx context.Context, b *bot.Bot, msg *models.Message) {
	sess, ok := s.sessions.get(msg.Chat.ID)
	if !ok || sess == nil || sess.FieldIndex < 0 {
		s.sendText(ctx, b, msg.Chat.ID, "Nothing to skip.")
		return
	}
	fields := s.cfg.fieldsForTarget(sess.TargetType)
	if sess.FieldIndex >= len(fields) {
		return
	}
	if fields[sess.FieldIndex].Required {
		s.sendText(ctx, b, msg.Chat.ID, "This field is required.")
		return
	}
	sess.FieldIndex++
	s.sessions.set(msg.Chat.ID, sess)
	if sess.FieldIndex >= len(fields) {
		if sess.Mode == modeClassify {
			s.finishClassify(ctx, b, msg.Chat.ID, sess)
		} else {
			s.finishFastPath(ctx, b, msg.Chat.ID, sess)
		}
		return
	}
	s.promptNextField(ctx, b, msg.Chat.ID, sess, fields)
}

func (s *botService) promptNextField(ctx context.Context, b *bot.Bot, chatID int64, sess *session, fields []fieldConfig) {
	for sess.FieldIndex < len(fields) {
		field := fields[sess.FieldIndex]
		if !field.Required {
			if v, ok := sess.Collected[field.Name]; ok && strings.TrimSpace(v) != "" {
				sess.FieldIndex++
				continue
			}
		}
		s.sendText(ctx, b, chatID, field.Prompt)
		return
	}
	if sess.Mode == modeClassify {
		s.finishClassify(ctx, b, chatID, sess)
	} else {
		s.finishFastPath(ctx, b, chatID, sess)
	}
}

func (s *botService) finishClassify(ctx context.Context, b *bot.Bot, chatID int64, sess *session) {
	payload, err := mergeFieldPayload(sess.Collected)
	if err != nil {
		s.sendText(ctx, b, chatID, "Could not classify capture.")
		return
	}
	result, err := classify.Classify(ctx, s.store, classify.ClassifyRequest{
		RecordRef:  sess.PendingRecordRef,
		TargetType: sess.TargetType,
		Payload:    payload,
	})
	if err != nil {
		log.Printf("telegram-source: classify %s: %v", sess.PendingRecordRef, err)
		s.sendText(ctx, b, chatID, fmt.Sprintf("Could not classify %s: %v", sess.PendingRecordRef, err))
		return
	}
	s.sessions.clear(chatID)
	s.sendText(ctx, b, chatID, fmt.Sprintf("Logged as %s (%s v%d)", sess.TargetType, result.RecordRef, result.Version))
}

func (s *botService) handleClassifyCommand(ctx context.Context, b *bot.Bot, msg *models.Message, args []string) {
	if len(args) < 2 {
		s.sendText(ctx, b, msg.Chat.ID, "Usage: /classify <record_ref> <target_type>")
		return
	}
	result, err := classify.Classify(ctx, s.store, classify.ClassifyRequest{
		RecordRef:  args[0],
		TargetType: args[1],
	})
	if err != nil {
		s.sendText(ctx, b, msg.Chat.ID, fmt.Sprintf("Classify failed: %v", err))
		return
	}
	s.sessions.clear(msg.Chat.ID)
	s.sendText(ctx, b, msg.Chat.ID, fmt.Sprintf("Logged as %s (%s v%d)", args[1], result.RecordRef, result.Version))
}

func (s *botService) startFastPath(ctx context.Context, b *bot.Bot, msg *models.Message, cmd commandConfig) {
	if _, busy := s.sessions.activePendingID(msg.Chat.ID); busy {
		s.sendText(ctx, b, msg.Chat.ID, "Finish the current capture or /cancel first.")
		return
	}
	parts := strings.Fields(msg.Text)
	if len(parts) > 1 {
		sess := &session{
			Mode:            modeFastPath,
			TargetType:      cmd.TargetType,
			AwaitingContent: false,
			FieldIndex:      -1,
			Collected:       make(map[string]string),
		}
		s.sessions.set(msg.Chat.ID, sess)
		clone := *msg
		clone.Text = strings.TrimSpace(strings.TrimPrefix(msg.Text, parts[0]))
		s.handleFastPathContent(ctx, b, &clone, sess)
		return
	}
	s.sessions.set(msg.Chat.ID, &session{
		Mode:            modeFastPath,
		TargetType:      cmd.TargetType,
		AwaitingContent: true,
		FieldIndex:      -1,
		Collected:       make(map[string]string),
	})
	s.sendText(ctx, b, msg.Chat.ID, fmt.Sprintf("Send content for %s.", cmd.Description))
}

func (s *botService) handleFastPathContent(ctx context.Context, b *bot.Bot, msg *models.Message, sess *session) {
	draft, err := s.buildDraft(ctx, b, msg)
	if err != nil {
		s.sendText(ctx, b, msg.Chat.ID, "Could not read that message.")
		return
	}
	sess.Draft = draft
	sess.AwaitingContent = false
	sess.Collected = fieldDefaultsFromDraft(draft)
	s.sessions.set(msg.Chat.ID, sess)

	fields := s.cfg.fieldsForTarget(sess.TargetType)
	if len(fields) == 0 {
		s.finishFastPath(ctx, b, msg.Chat.ID, sess)
		return
	}
	sess.FieldIndex = 0
	s.sessions.set(msg.Chat.ID, sess)
	s.promptNextField(ctx, b, msg.Chat.ID, sess, fields)
}

func (s *botService) finishFastPath(ctx context.Context, b *bot.Bot, chatID int64, sess *session) {
	if sess.Draft == nil {
		s.sendText(ctx, b, chatID, "Nothing to save.")
		return
	}
	payload := map[string]any{}
	if len(sess.Draft.CaptureJSON) > 0 {
		_ = json.Unmarshal(sess.Draft.CaptureJSON, &payload)
	}
	for k, v := range sess.Collected {
		payload[k] = v
	}
	delete(payload, "time")
	delete(payload, "blob_ref")
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		s.sendText(ctx, b, chatID, "Could not save.")
		return
	}
	event := &troverpc.Event{
		Type:    sess.TargetType,
		Source:  "telegram",
		Time:    sess.Draft.Time,
		BlobRef: sess.Draft.BlobRef,
		Payload: payloadBytes,
	}
	if _, err := trovemodule.EmitRecordFromEvent(ctx, s.core, event); err != nil {
		log.Printf("telegram-source: emit fast path: %v", err)
		s.sendText(ctx, b, chatID, "Could not save.")
		return
	}
	s.sessions.clear(chatID)
	s.sendText(ctx, b, chatID, fmt.Sprintf("Logged as %s.", sess.TargetType))
}

func (s *botService) buildDraft(ctx context.Context, b *bot.Bot, msg *models.Message) (*captureDraft, error) {
	var (
		kind   string
		text   = extractMessageText(msg)
		fileID string
		file   *telegramFileMeta
	)

	switch {
	case len(msg.Photo) > 0:
		kind = "photo"
		fileID = largestPhotoFileID(msg)
		file = fileMetaFromPhoto(fileID, int64(msg.Photo[len(msg.Photo)-1].FileSize))
	case msg.Document != nil:
		kind = "document"
		fileID = msg.Document.FileID
		file = fileMetaFromDocument(msg.Document)
	case msg.Voice != nil:
		kind = "voice"
		fileID = msg.Voice.FileID
		file = fileMetaFromVoice(msg.Voice)
	case text != "":
		kind = "text"
	default:
		return nil, fmt.Errorf("unsupported message content")
	}

	var blobRef string
	if fileID != "" {
		data, err := downloadTelegramFile(ctx, b, fileID, s.cfg.MaxFileBytes)
		if err != nil {
			return nil, err
		}
		ref, err := s.core.Put(ctx, data)
		if err != nil {
			return nil, err
		}
		blobRef = ref
	}

	body, err := buildCaptureBody(msg, kind, text, blobRef, file)
	if err != nil {
		return nil, err
	}
	return &captureDraft{
		Time:        messageTime(msg),
		BlobRef:     blobRef,
		Text:        text,
		CaptureJSON: body,
	}, nil
}

func (s *botService) typeKeyboard() *models.InlineKeyboardMarkup {
	row := make([]models.InlineKeyboardButton, 0, len(s.cfg.Types))
	for i, opt := range s.cfg.Types {
		row = append(row, models.InlineKeyboardButton{
			Text:         opt.Label,
			CallbackData: fmt.Sprintf("%s%d", callbackTypePrefix, i),
		})
	}
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{row},
	}
}

func (s *botService) sendText(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	if b == nil {
		return
	}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
}

func (s *botService) sendTextWithKeyboard(ctx context.Context, b *bot.Bot, chatID int64, text string, kb *models.InlineKeyboardMarkup) {
	if b == nil {
		return
	}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: kb,
	})
}
