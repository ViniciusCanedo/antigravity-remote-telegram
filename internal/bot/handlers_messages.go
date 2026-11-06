package bot

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"antigravity-telegram-remote/internal/bot/ui"
	"antigravity-telegram-remote/internal/bridge"
	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/logger"
	"antigravity-telegram-remote/internal/voice"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type voiceFileGetterAdapter struct {
	resolver TelegramFileResolver
}

func (a voiceFileGetterAdapter) GetFile(ctx context.Context, fileID string) (*voice.TelegramFileInfo, error) {
	fp, err := a.resolver.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	return &voice.TelegramFileInfo{FilePath: fp}, nil
}

// SetFileResolver overrides the file resolver used for retrieving photos and voice files.
func (b *Bot) SetFileResolver(r TelegramFileResolver) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.fileResolver = r
}

func (b *Bot) getFileResolver() TelegramFileResolver {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	if b.fileResolver != nil {
		return b.fileResolver
	}
	return b
}

// GetFile retrieves the file path for a given fileID. Implements TelegramFileResolver.
func (b *Bot) GetFile(ctx context.Context, fileID string) (string, error) {
	b.stateMu.RLock()
	resolver := b.fileResolver
	b.stateMu.RUnlock()
	if resolver != nil && resolver != b {
		return resolver.GetFile(ctx, fileID)
	}

	if b.TgBot == nil {
		return "", fmt.Errorf("bot client is nil")
	}
	file, err := b.TgBot.GetFile(ctx, &tgbot.GetFileParams{FileID: fileID})
	if err != nil {
		return "", err
	}
	if file == nil || file.FilePath == "" {
		return "", fmt.Errorf("empty file_path returned by Telegram")
	}
	return file.FilePath, nil
}

// Reply sends a plain text message to the given Telegram channel.
func (b *Bot) Reply(ctx context.Context, ch TelegramChannel, text string) error {
	sender := b.getSender()
	if sender == nil {
		return nil
	}
	params := &tgbot.SendMessageParams{
		ChatID: ch.ChatID,
		Text:   text,
	}
	if ch.ThreadID != 0 {
		params.MessageThreadID = ch.ThreadID
	}
	_, err := sender.SendMessage(ctx, params)
	return err
}

func (b *Bot) threadIDPtr(ch TelegramChannel) *int {
	if ch.ThreadID != 0 {
		th := ch.ThreadID
		return &th
	}
	return nil
}

func (b *Bot) getCurrentCdp() bridge.CdpEvaluator {
	if b.Bridge == nil || b.Pool == nil {
		return nil
	}
	if b.Bridge.LastActiveWorkspace != nil && *b.Bridge.LastActiveWorkspace != "" {
		client := b.Pool.GetConnected(*b.Bridge.LastActiveWorkspace)
		if client != nil {
			return &cdpServiceWithContextsAdapter{client: client}
		}
	}
	activeNames := b.Pool.GetActiveWorkspaceNames()
	if len(activeNames) > 0 {
		client := b.Pool.GetConnected(activeNames[0])
		if client != nil {
			return &cdpServiceWithContextsAdapter{client: client}
		}
	}
	return nil
}

func (b *Bot) getPromptDispatchOptions() *bridge.PromptDispatchOptions {
	return &bridge.PromptDispatchOptions{
		ChatSessionService: b.ChatSessionService,
		ChatSessionRepo:    b.ChatSessionRepo,
		TopicManager:       b.TopicManager,
		TitleGenerator:     b.TitleGenerator,
	}
}

func (b *Bot) handleScreenshotAction(ctx context.Context, ch TelegramChannel) error {
	var cdpClient cdp.PoolClient
	if b.Pool != nil && b.Bridge != nil && b.Bridge.LastActiveWorkspace != nil {
		cdpClient = b.Pool.GetConnected(*b.Bridge.LastActiveWorkspace)
	}

	sendPhotoFn := func(c context.Context, filename string, photo []byte, caption string) error {
		sender := b.getSender()
		if sender == nil {
			return nil
		}
		params := &tgbot.SendPhotoParams{
			ChatID:  ch.ChatID,
			Caption: caption,
			Photo: &models.InputFileUpload{
				Filename: filename,
				Data:     bytes.NewReader(photo),
			},
		}
		if ch.ThreadID != 0 {
			params.MessageThreadID = ch.ThreadID
		}
		_, err := sender.SendPhoto(c, params)
		return err
	}

	replyFn := func(c context.Context, text string) error {
		return b.Reply(c, ch, text)
	}

	var capturer ui.ScreenshotCapturer
	if cdpClient != nil {
		capturer = screenshotCapturerAdapter{client: cdpClient}
	}

	return ui.HandleScreenshot(ctx, sendPhotoFn, replyFn, capturer)
}

// HandleTextMessage processes incoming text messages including pending plan edits, slash commands, and prompts.
func (b *Bot) HandleTextMessage(ctx context.Context, ch TelegramChannel, rawText string) error {
	trimmed := strings.TrimSpace(rawText)
	if trimmed == "" {
		return nil
	}

	key := ChannelKey(ch)

	// 1. Plan edit interception
	if _, ok := b.GetPlanEditPending(key); ok {
		b.ClearPlanEditPending(key)
		if trimmed == "/cancel" {
			return b.Reply(ctx, ch, "Plan edit cancelled.")
		}

		editPrompt := fmt.Sprintf("Please revise the plan based on the following feedback:\n\n%s", trimmed)
		resolved := b.ResolveWorkspaceAndCdp(ctx, ch)
		var cdpEval bridge.CdpEvaluator
		if resolved != nil && resolved.Cdp != nil {
			cdpEval = &cdpServiceWithContextsAdapter{client: resolved.Cdp}
		} else if cur := b.getCurrentCdp(); cur != nil {
			cdpEval = cur
		}

		if cdpEval == nil {
			return b.Reply(ctx, ch, "Not connected to CDP.")
		}

		_ = b.Reply(ctx, ch, "Sending plan edit...")
		if b.PromptDispatcher != nil {
			return b.PromptDispatcher.Send(ctx, bridge.PromptDispatchRequest{
				Channel:       bridge.TelegramChannel{ChatID: ch.ChatID, ThreadID: b.threadIDPtr(ch)},
				Prompt:        editPrompt,
				Cdp:           cdpEval,
				InboundImages: []bridge.InboundImageAttachment{},
				Options:       b.getPromptDispatchOptions(),
			})
		}
		return nil
	}

	// 2. Check if message is a slash command
	parsed := ParseMessageContent(trimmed)
	if parsed.IsCommand && parsed.CommandName != "" {
		cmdLower := strings.ToLower(parsed.CommandName)
		if cmdLower == "autoaccept" {
			var action string
			if len(parsed.Args) > 0 {
				action = parsed.Args[0]
			}
			var res bridge.AutoAcceptResult
			if b.Bridge != nil && b.Bridge.AutoAccept != nil {
				res, _ = b.Bridge.AutoAccept.Handle(action)
			}
			return b.Reply(ctx, ch, res.Message)
		}

		if cmdLower == "screenshot" {
			return b.handleScreenshotAction(ctx, ch)
		}

		if cmdLower == "status" {
			activeNames := []string{}
			if b.Pool != nil {
				activeNames = b.Pool.GetActiveWorkspaceNames()
			}
			currentMode := "fast"
			if b.ModeService != nil {
				currentMode = string(b.ModeService.GetCurrentMode())
			}
			modeDisplay := ui.ModeDisplayNames[currentMode]
			if modeDisplay == "" {
				modeDisplay = currentMode
			}
			cdpStatus := "⚪ Disconnected"
			if len(activeNames) > 0 {
				cdpStatus = fmt.Sprintf("🟢 %d project(s)", len(activeNames))
			}
			autoApproveStatus := "⚪ OFF"
			if b.Bridge != nil && b.Bridge.AutoAccept != nil && b.Bridge.AutoAccept.IsEnabled() {
				autoApproveStatus = "🟢 ON"
			}
			statusText := fmt.Sprintf("<b>🔧 Bot Status</b>\n\n<b>CDP:</b> %s\n<b>Mode:</b> %s\n<b>Auto Approve:</b> %s",
				cdpStatus, ui.EscapeHTML(modeDisplay), autoApproveStatus)
			return b.ReplyHTML(ctx, ch, statusText, nil)
		}

		if b.SlashCommandHandler != nil {
			result := b.SlashCommandHandler.HandleCommand(parsed.CommandName, parsed.Args)
			_ = b.Reply(ctx, ch, result.Message)

			if result.Prompt != "" {
				var cdpEval bridge.CdpEvaluator
				if resolved := b.ResolveWorkspaceAndCdp(ctx, ch); resolved != nil && resolved.Cdp != nil {
					cdpEval = &cdpServiceWithContextsAdapter{client: resolved.Cdp}
				} else if cur := b.getCurrentCdp(); cur != nil {
					cdpEval = cur
				}

				if cdpEval != nil && b.PromptDispatcher != nil {
					return b.PromptDispatcher.Send(ctx, bridge.PromptDispatchRequest{
						Channel:       bridge.TelegramChannel{ChatID: ch.ChatID, ThreadID: b.threadIDPtr(ch)},
						Prompt:        result.Prompt,
						Cdp:           cdpEval,
						InboundImages: []bridge.InboundImageAttachment{},
						Options:       b.getPromptDispatchOptions(),
					})
				} else if cdpEval == nil {
					return b.Reply(ctx, ch, "Not connected to CDP. Send a message first to connect to a project.")
				}
			}
		}
		return nil
	}

	// 3. Regular message — route to Antigravity
	resolved := b.ResolveWorkspaceAndCdp(ctx, ch)
	if resolved == nil {
		return b.Reply(ctx, ch, "No project is configured for this chat. Use /project to select one.")
	}

	var session *database.ChatSessionRecord
	if b.ChatSessionRepo != nil {
		session, _ = b.ChatSessionRepo.FindByChannelId(key)
	}

	bridgeCh := bridge.TelegramChannel{ChatID: ch.ChatID, ThreadID: b.threadIDPtr(ch)}
	var dispName string
	if session != nil && session.DisplayName != nil {
		dispName = *session.DisplayName
	}
	if dispName != "" {
		bridge.RegisterApprovalSessionChannel(b.Bridge, resolved.ProjectName, dispName, bridgeCh)
	}

	if sessionClient, ok := resolved.Cdp.(cdp.SessionCDPClient); ok && b.ChatSessionService != nil {
		if session != nil && session.IsRenamed && dispName != "" {
			activationResult := b.ChatSessionService.ActivateSessionByTitle(sessionClient, dispName)
			if !activationResult.OK {
				return b.Reply(ctx, ch, fmt.Sprintf("⚠️ Could not route to session (%s).", dispName))
			}
		} else if session != nil && !session.IsRenamed {
			_ = b.ChatSessionService.StartNewChat(sessionClient)
		}
	}

	if b.Bridge != nil && b.Bridge.Pool != nil {
		userMsgDetector := b.Bridge.Pool.GetUserMessageDetector(resolved.ProjectName)
		if userMsgDetector != nil {
			userMsgDetector.AddEchoHash(trimmed)
		}
	}

	if b.PromptDispatcher != nil {
		cdpAdapter := &cdpServiceWithContextsAdapter{client: resolved.Cdp}
		return b.PromptDispatcher.Send(ctx, bridge.PromptDispatchRequest{
			Channel:       bridgeCh,
			Prompt:        trimmed,
			Cdp:           cdpAdapter,
			InboundImages: []bridge.InboundImageAttachment{},
			Options:       b.getPromptDispatchOptions(),
		})
	}
	return nil
}

// HandlePhotoMessage processes image attachments, downloading the highest resolution photo and dispatching to Antigravity.
func (b *Bot) HandlePhotoMessage(
	ctx context.Context,
	ch TelegramChannel,
	messageID int,
	photos []TelegramPhotoInfo,
	caption string,
) error {
	if len(photos) == 0 {
		return nil
	}

	largest := photos[len(photos)-1]
	prompt := strings.TrimSpace(caption)
	if prompt == "" {
		prompt = "Please review the attached images and respond accordingly."
	}

	resolved := b.ResolveWorkspaceAndCdp(ctx, ch)
	if resolved == nil {
		return b.Reply(ctx, ch, "No project configured. Use /project first.")
	}

	botToken := ""
	if b.Config != nil {
		botToken = b.Config.TelegramBotToken
	}

	resolver := b.getFileResolver()
	inboundImages, err := DownloadTelegramImages(ctx, resolver, botToken, []TelegramPhotoInfo{largest}, fmt.Sprintf("%d", messageID), b.httpClient)
	if err != nil {
		logger.Error(fmt.Sprintf("[HandlePhotoMessage] Download failed: %v", err))
		return b.Reply(ctx, ch, "Failed to download image.")
	}
	defer CleanupInboundImageAttachments(inboundImages)

	bridgeInbound := make([]bridge.InboundImageAttachment, 0, len(inboundImages))
	for _, img := range inboundImages {
		bridgeInbound = append(bridgeInbound, bridge.InboundImageAttachment{
			FileName:  img.Name,
			MimeType:  img.MimeType,
			LocalPath: img.LocalPath,
			URL:       img.URL,
		})
	}

	bridgeCh := bridge.TelegramChannel{ChatID: ch.ChatID, ThreadID: b.threadIDPtr(ch)}
	if b.PromptDispatcher != nil {
		cdpAdapter := &cdpServiceWithContextsAdapter{client: resolved.Cdp}
		return b.PromptDispatcher.Send(ctx, bridge.PromptDispatchRequest{
			Channel:       bridgeCh,
			Prompt:        prompt,
			Cdp:           cdpAdapter,
			InboundImages: bridgeInbound,
			Options:       b.getPromptDispatchOptions(),
		})
	}
	return nil
}

// HandleVoiceMessage transcribes voice audio via Whisper and processes the transcription as a command or prompt.
func (b *Bot) HandleVoiceMessage(
	ctx context.Context,
	ch TelegramChannel,
	voiceInfo voice.TelegramVoiceInfo,
) error {
	var whisperIssue *string
	if b.whisperInspector != nil {
		whisperIssue = voice.CheckWhisperAvailability(*b.whisperInspector)
	} else {
		whisperIssue = voice.CheckWhisperAvailability()
	}
	if whisperIssue != nil && *whisperIssue != "" {
		return b.Reply(ctx, ch, *whisperIssue)
	}

	resolved := b.ResolveWorkspaceAndCdp(ctx, ch)
	if resolved == nil {
		return b.Reply(ctx, ch, "No project configured. Use /project first.")
	}

	_ = b.Reply(ctx, ch, "🎙️ Transcribing voice message...")

	botToken := ""
	if b.Config != nil {
		botToken = b.Config.TelegramBotToken
	}

	resolver := b.getFileResolver()
	voicePath, err := voice.DownloadTelegramVoice(ctx, voiceFileGetterAdapter{resolver: resolver}, botToken, voiceInfo, b.httpClient)
	if err != nil {
		logger.Error(fmt.Sprintf("[Voice] Download failed: %v", err))
		return b.Reply(ctx, ch, "❌ Could not download voice message. Please try again.")
	}

	var transcriptPtr *string
	if b.whisperRunner != nil {
		transcriptPtr = voice.TranscribeVoice(ctx, voicePath, b.whisperRunner)
	} else {
		transcriptPtr = voice.TranscribeVoice(ctx, voicePath)
	}
	if transcriptPtr == nil || strings.TrimSpace(*transcriptPtr) == "" {
		return b.Reply(ctx, ch, "❌ Could not transcribe voice message. Please try again or type your prompt.")
	}

	transcript := strings.TrimSpace(*transcriptPtr)

	// Check if transcription is a slash command
	parsed := ParseMessageContent(transcript)
	if parsed.IsCommand && parsed.CommandName != "" {
		if b.SlashCommandHandler != nil {
			result := b.SlashCommandHandler.HandleCommand(parsed.CommandName, parsed.Args)
			_ = b.Reply(ctx, ch, fmt.Sprintf("🎙️ \"%s\"\n\n%s", transcript, result.Message))

			if result.Prompt != "" {
				cdpEval := b.getCurrentCdp()
				if cdpEval != nil && b.PromptDispatcher != nil {
					return b.PromptDispatcher.Send(ctx, bridge.PromptDispatchRequest{
						Channel:       bridge.TelegramChannel{ChatID: ch.ChatID, ThreadID: b.threadIDPtr(ch)},
						Prompt:        result.Prompt,
						Cdp:           cdpEval,
						InboundImages: []bridge.InboundImageAttachment{},
						Options:       b.getPromptDispatchOptions(),
					})
				}
			}
		}
		return nil
	}

	_ = b.Reply(ctx, ch, fmt.Sprintf("📝 \"%s\"", transcript))

	if b.Bridge != nil && b.Bridge.Pool != nil {
		userMsgDetector := b.Bridge.Pool.GetUserMessageDetector(resolved.ProjectName)
		if userMsgDetector != nil {
			userMsgDetector.AddEchoHash(transcript)
		}
	}

	if b.PromptDispatcher != nil {
		bridgeCh := bridge.TelegramChannel{ChatID: ch.ChatID, ThreadID: b.threadIDPtr(ch)}
		cdpAdapter := &cdpServiceWithContextsAdapter{client: resolved.Cdp}
		return b.PromptDispatcher.Send(ctx, bridge.PromptDispatchRequest{
			Channel:       bridgeCh,
			Prompt:        transcript,
			Cdp:           cdpAdapter,
			InboundImages: []bridge.InboundImageAttachment{},
			Options:       b.getPromptDispatchOptions(),
		})
	}
	return nil
}

// RegisterMessageHandlers registers callback query and message event handlers on TgBot.
func (b *Bot) RegisterMessageHandlers() {
	if b.TgBot == nil {
		return
	}

	// 1. Callback query router
	b.TgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return update.CallbackQuery != nil && update.CallbackQuery.Data != ""
	}, func(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
		cbCtx := NewTelegramCallbackContext(b, update)
		_ = b.HandleCallbackQuery(ctx, cbCtx)
	})

	// 2. Photo message handler
	b.TgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return update.Message != nil && len(update.Message.Photo) > 0
	}, func(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
		ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
		photos := make([]TelegramPhotoInfo, 0, len(update.Message.Photo))
		for _, p := range update.Message.Photo {
			photos = append(photos, TelegramPhotoInfo{
				FileID:       p.FileID,
				FileUniqueID: p.FileUniqueID,
				Width:        p.Width,
				Height:       p.Height,
				FileSize:     int64(p.FileSize),
			})
		}
		_ = b.HandlePhotoMessage(ctx, ch, update.Message.ID, photos, update.Message.Caption)
	})

	// 3. Voice message handler
	b.TgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return update.Message != nil && update.Message.Voice != nil
	}, func(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
		ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
		v := update.Message.Voice
		vInfo := voice.TelegramVoiceInfo{
			FileID:       v.FileID,
			FileUniqueID: v.FileUniqueID,
			Duration:     v.Duration,
			MimeType:     v.MimeType,
			FileSize:     v.FileSize,
		}
		_ = b.HandleVoiceMessage(ctx, ch, vInfo)
	})

	// 4. Text message handler
	b.TgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return update.Message != nil && update.Message.Text != ""
	}, func(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
		ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
		_ = b.HandleTextMessage(ctx, ch, update.Message.Text)
	})
}
