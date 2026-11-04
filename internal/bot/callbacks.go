package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"antigravity-telegram-remote/internal/bot/ui"
	"antigravity-telegram-remote/internal/bridge"
	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/logger"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// CallbackContext abstracts interaction data and methods for answering callback queries.
type CallbackContext interface {
	Data() string
	Channel() TelegramChannel
	ChatType() string
	IsForum() bool
	EditMessageText(ctx context.Context, text string, keyboard *ui.InlineKeyboardMarkup) error
	EditMessagePlainText(ctx context.Context, text string) error
	RemoveInlineKeyboard(ctx context.Context) error
	AnswerCallbackQuery(ctx context.Context, text string, showAlert bool) error
}

// TelegramCallbackContext wraps a native tgbot update into CallbackContext.
type TelegramCallbackContext struct {
	bot    *Bot
	update *models.Update
}

// NewTelegramCallbackContext constructs a new TelegramCallbackContext.
func NewTelegramCallbackContext(bot *Bot, update *models.Update) *TelegramCallbackContext {
	return &TelegramCallbackContext{bot: bot, update: update}
}

func (c *TelegramCallbackContext) Data() string {
	if c.update != nil && c.update.CallbackQuery != nil {
		return c.update.CallbackQuery.Data
	}
	return ""
}

func (c *TelegramCallbackContext) Channel() TelegramChannel {
	if c.update != nil && c.update.CallbackQuery != nil && c.update.CallbackQuery.Message.Message != nil {
		msg := c.update.CallbackQuery.Message.Message
		return GetChannelFromCallback(msg.Chat.ID, msg.MessageThreadID)
	}
	return TelegramChannel{}
}

func (c *TelegramCallbackContext) ChatType() string {
	if c.update != nil && c.update.CallbackQuery != nil && c.update.CallbackQuery.Message.Message != nil {
		return string(c.update.CallbackQuery.Message.Message.Chat.Type)
	}
	return ""
}

func (c *TelegramCallbackContext) IsForum() bool {
	if c.update != nil && c.update.CallbackQuery != nil && c.update.CallbackQuery.Message.Message != nil {
		return c.update.CallbackQuery.Message.Message.Chat.IsForum
	}
	return false
}

func (c *TelegramCallbackContext) EditMessageText(ctx context.Context, text string, keyboard *ui.InlineKeyboardMarkup) error {
	sender := c.bot.getSender()
	if sender == nil || c.update == nil || c.update.CallbackQuery == nil || c.update.CallbackQuery.Message.Message == nil {
		return nil
	}
	msg := c.update.CallbackQuery.Message.Message
	params := &tgbot.EditMessageTextParams{
		ChatID:    msg.Chat.ID,
		MessageID: msg.ID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}
	if keyboard != nil {
		params.ReplyMarkup = keyboard.ToTelegramModel()
	}
	_, err := sender.EditMessageText(ctx, params)
	return err
}

func (c *TelegramCallbackContext) EditMessagePlainText(ctx context.Context, text string) error {
	sender := c.bot.getSender()
	if sender == nil || c.update == nil || c.update.CallbackQuery == nil || c.update.CallbackQuery.Message.Message == nil {
		return nil
	}
	msg := c.update.CallbackQuery.Message.Message
	params := &tgbot.EditMessageTextParams{
		ChatID:    msg.Chat.ID,
		MessageID: msg.ID,
		Text:      text,
	}
	_, err := sender.EditMessageText(ctx, params)
	return err
}

func (c *TelegramCallbackContext) RemoveInlineKeyboard(ctx context.Context) error {
	sender := c.bot.getSender()
	if sender == nil || c.update == nil || c.update.CallbackQuery == nil || c.update.CallbackQuery.Message.Message == nil {
		return nil
	}
	msg := c.update.CallbackQuery.Message.Message
	params := &tgbot.EditMessageReplyMarkupParams{
		ChatID:      msg.Chat.ID,
		MessageID:   msg.ID,
		ReplyMarkup: nil,
	}
	_, err := sender.EditMessageReplyMarkup(ctx, params)
	return err
}

func (c *TelegramCallbackContext) AnswerCallbackQuery(ctx context.Context, text string, showAlert bool) error {
	sender := c.bot.getSender()
	if sender == nil || c.update == nil || c.update.CallbackQuery == nil {
		return nil
	}
	params := &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: c.update.CallbackQuery.ID,
		Text:            text,
		ShowAlert:       showAlert,
	}
	_, err := sender.AnswerCallbackQuery(ctx, params)
	return err
}

// HandleCallbackQuery processes all 11 families of callback_data.
func (b *Bot) HandleCallbackQuery(ctx context.Context, cb CallbackContext) error {
	if cb == nil {
		return nil
	}
	data := cb.Data()
	ch := cb.Channel()

	// 1. Mode selection: mode_select:<mode>
	if strings.HasPrefix(data, "mode_select:") {
		selectedMode := strings.TrimPrefix(data, "mode_select:")
		if b.ModeService != nil {
			_ = b.ModeService.SetMode(selectedMode)
		}
		var currentClient cdp.PoolClient
		if b.Pool != nil && b.Bridge != nil && b.Bridge.LastActiveWorkspace != nil {
			currentClient = b.Pool.GetConnected(*b.Bridge.LastActiveWorkspace)
		}
		if currentClient != nil {
			if c, ok := currentClient.(*cdp.Client); ok && c != nil {
				res, err := c.SetUIMode(selectedMode)
				if err != nil || !res.OK {
					logger.Warn(fmt.Sprintf("[Mode] UI switch failed: %s", res.Error))
				}
			}
		}
		var modeCdp ui.ModeCdpProvider
		if currentClient != nil {
			modeCdp = modeCdpUIAdapter{client: currentClient}
		}
		uiPayload, err := ui.BuildModeUI(ctx, modeServiceUIAdapter{svc: b.ModeService}, &ui.ModeUIDeps{
			GetCurrentCdp: func() ui.ModeCdpProvider { return modeCdp },
		})
		if err == nil {
			_ = cb.EditMessageText(ctx, uiPayload.Text, uiPayload.Keyboard)
		}
		displayName := ui.ModeDisplayNames[selectedMode]
		if displayName == "" {
			displayName = selectedMode
		}
		return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("Mode: %s", displayName), false)
	}

	// 2. Exhausted model toast alert: model_exhausted_<modelName>
	if strings.HasPrefix(data, ui.ModelExhaustedPrefix) {
		modelName := strings.TrimPrefix(data, ui.ModelExhaustedPrefix)
		return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("⛔ %s is exhausted. Wait for quota reset or pick another model.", modelName), true)
	}

	// 3. Model selection: model_btn_<modelName>
	if strings.HasPrefix(data, ui.ModelBtnPrefix) {
		modelName := strings.TrimPrefix(data, ui.ModelBtnPrefix)
		var currentClient cdp.PoolClient
		if b.Pool != nil && b.Bridge != nil && b.Bridge.LastActiveWorkspace != nil {
			currentClient = b.Pool.GetConnected(*b.Bridge.LastActiveWorkspace)
		}
		if currentClient == nil {
			return cb.AnswerCallbackQuery(ctx, "Not connected to CDP.", false)
		}
		if c, ok := currentClient.(*cdp.Client); ok && c != nil {
			res, err := c.SetUIModel(modelName)
			if err == nil && res.OK {
				var fetchQuotaFn func(ctx context.Context) ([]ui.ModelQuotaEntry, error)
				if b.Bridge != nil && b.Bridge.Quota != nil {
					fetchQuotaFn = func(ctx context.Context) ([]ui.ModelQuotaEntry, error) {
						q, qErr := b.Bridge.Quota.FetchQuota(ctx)
						if qErr != nil {
							return nil, qErr
						}
						if list, ok := q.([]ui.ModelQuotaEntry); ok {
							return list, nil
						}
						return nil, nil
					}
				}
				modelsUI, err := ui.BuildModelsUI(ctx, modelsCdpUIAdapter{client: currentClient}, fetchQuotaFn)
				if err == nil && modelsUI != nil {
					_ = cb.EditMessageText(ctx, modelsUI.Text, modelsUI.Keyboard)
				}
				retModel := res.Model
				if retModel == "" {
					retModel = modelName
				}
				return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("Model: %s", retModel), false)
			}
			errMsg := "Failed to change model."
			if res.Error != "" {
				errMsg = res.Error
			} else if err != nil {
				errMsg = err.Error()
			}
			return cb.AnswerCallbackQuery(ctx, errMsg, false)
		}
		return cb.AnswerCallbackQuery(ctx, "Failed to change model.", false)
	}

	// 4. Model refresh: model_refresh_btn
	if data == ui.ModelRefreshBtn {
		var currentClient cdp.PoolClient
		if b.Pool != nil && b.Bridge != nil && b.Bridge.LastActiveWorkspace != nil {
			currentClient = b.Pool.GetConnected(*b.Bridge.LastActiveWorkspace)
		}
		if currentClient == nil {
			return cb.AnswerCallbackQuery(ctx, "Not connected.", false)
		}
		var fetchQuotaFn func(ctx context.Context) ([]ui.ModelQuotaEntry, error)
		if b.Bridge != nil && b.Bridge.Quota != nil {
			fetchQuotaFn = func(ctx context.Context) ([]ui.ModelQuotaEntry, error) {
				q, qErr := b.Bridge.Quota.FetchQuota(ctx)
				if qErr != nil {
					return nil, qErr
				}
				if list, ok := q.([]ui.ModelQuotaEntry); ok {
					return list, nil
				}
				return nil, nil
			}
		}
		modelsUI, err := ui.BuildModelsUI(ctx, modelsCdpUIAdapter{client: currentClient}, fetchQuotaFn)
		if err == nil && modelsUI != nil {
			_ = cb.EditMessageText(ctx, modelsUI.Text, modelsUI.Keyboard)
		}
		return cb.AnswerCallbackQuery(ctx, "Refreshed", false)
	}

	// 5. Auto-accept buttons
	if data == ui.AutoAcceptBtnOn || data == ui.AutoAcceptBtnOff {
		action := "on"
		if data == ui.AutoAcceptBtnOff {
			action = "off"
		}
		if b.Bridge != nil && b.Bridge.AutoAccept != nil {
			_, _ = b.Bridge.AutoAccept.Handle(action)
			uiPayload := ui.BuildAutoAcceptUI(b.Bridge.AutoAccept)
			_ = cb.EditMessageText(ctx, uiPayload.Text, uiPayload.Keyboard)
		}
		return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("Auto-accept: %s", strings.ToUpper(action)), false)
	}

	if data == ui.AutoAcceptBtnRefresh {
		if b.Bridge != nil && b.Bridge.AutoAccept != nil {
			uiPayload := ui.BuildAutoAcceptUI(b.Bridge.AutoAccept)
			_ = cb.EditMessageText(ctx, uiPayload.Text, uiPayload.Keyboard)
		}
		return cb.AnswerCallbackQuery(ctx, "Refreshed", false)
	}

	// 6. Project selection & pagination
	if strings.HasPrefix(data, ui.ProjectSelectID+":") || strings.HasPrefix(data, ui.WorkspaceSelectID+":") {
		prefix := ui.ProjectSelectID + ":"
		if strings.HasPrefix(data, ui.WorkspaceSelectID+":") {
			prefix = ui.WorkspaceSelectID + ":"
		}
		workspacePath := strings.TrimPrefix(data, prefix)
		exists := false
		if b.WorkspaceService != nil {
			exists, _ = b.WorkspaceService.Exists(workspacePath)
		}
		if !exists {
			return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("Project \"%s\" not found.", workspacePath), false)
		}

		key := ChannelKey(ch)
		guildID := fmt.Sprintf("%d", ch.ChatID)
		isForum := cb.ChatType() == "supergroup" && cb.IsForum()

		// Auto-create forum topic if useTopics is true, chat is a forum, and no thread is active
		if b.Config != nil && b.Config.UseTopics && isForum && ch.ThreadID == 0 && b.TopicManager != nil {
			var existingTopicID int
			if b.WorkspaceBindingRepo != nil {
				bindings, _ := b.WorkspaceBindingRepo.FindByWorkspacePathAndGuildId(workspacePath, guildID)
				for _, rec := range bindings {
					if strings.Contains(rec.ChannelID, ":") {
						parts := strings.Split(rec.ChannelID, ":")
						if len(parts) == 2 {
							if id, err := strconv.Atoi(parts[1]); err == nil && id != 0 {
								existingTopicID = id
								break
							}
						}
					}
				}
			}

			topicID := existingTopicID
			if topicID != 0 {
				b.TopicManager.RegisterTopic(workspacePath, topicID)
			} else {
				b.TopicManager.SetChatID(ch.ChatID)
				sanitized := b.TopicManager.SanitizeName(workspacePath)
				result, err := b.TopicManager.EnsureTopic(ctx, sanitized)
				if err == nil {
					topicID = result.TopicID
				}
			}

			if topicID != 0 {
				key = fmt.Sprintf("%d:%d", ch.ChatID, topicID)
				fullPath, _ := b.WorkspaceService.GetWorkspacePath(workspacePath)
				welcomeText := fmt.Sprintf("<b>📁 Project Selected</b>\n\n✅ <b>%s</b>\n<code>%s</code>\n\nSend messages here to interact with this project.",
					ui.EscapeHTML(workspacePath), ui.EscapeHTML(fullPath))
				_ = b.ReplyHTML(ctx, TelegramChannel{ChatID: ch.ChatID, ThreadID: topicID}, welcomeText, nil)
				if b.WorkspaceBindingRepo != nil {
					_, _ = b.WorkspaceBindingRepo.Upsert(database.CreateWorkspaceBindingInput{
						ChannelID:     key,
						WorkspacePath: workspacePath,
						GuildID:       guildID,
					})
				}
				return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("Topic created for: %s", workspacePath), false)
			}
		}

		if b.WorkspaceBindingRepo != nil {
			_, _ = b.WorkspaceBindingRepo.Upsert(database.CreateWorkspaceBindingInput{
				ChannelID:     key,
				WorkspacePath: workspacePath,
				GuildID:       guildID,
			})
		}
		fullPath, _ := b.WorkspaceService.GetWorkspacePath(workspacePath)
		text := fmt.Sprintf("<b>📁 Project Selected</b>\n\n✅ <b>%s</b>\n<code>%s</code>\n\nSend messages here to interact with this project.",
			ui.EscapeHTML(workspacePath), ui.EscapeHTML(fullPath))
		_ = cb.EditMessageText(ctx, text, nil)
		return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("Selected: %s", workspacePath), false)
	}

	if strings.HasPrefix(data, ui.ProjectPagePrefix+":") {
		page, ok := ui.ParseProjectPageID(data)
		if ok && b.WorkspaceService != nil {
			workspaces := b.WorkspaceService.ScanWorkspaces()
			payload := ui.BuildProjectListUI(workspaces, page)
			_ = cb.EditMessageText(ctx, payload.Text, payload.Keyboard)
		}
		return cb.AnswerCallbackQuery(ctx, "", false)
	}

	// 7. Template button: template_btn_<id>
	if strings.HasPrefix(data, ui.TemplateBtnPrefix) {
		templateID, ok := ui.ParseTemplateButtonID(data)
		if !ok {
			return cb.AnswerCallbackQuery(ctx, "Invalid template.", false)
		}
		if b.TemplateRepo == nil {
			return cb.AnswerCallbackQuery(ctx, "Template not found.", false)
		}
		template, err := b.TemplateRepo.FindById(int64(templateID))
		if err != nil || template == nil {
			return cb.AnswerCallbackQuery(ctx, "Template not found.", false)
		}

		resolved := b.ResolveWorkspaceAndCdp(ctx, ch)
		var cdpClient cdp.PoolClient
		if resolved != nil {
			cdpClient = resolved.Cdp
		} else if b.Pool != nil && b.Bridge != nil && b.Bridge.LastActiveWorkspace != nil {
			cdpClient = b.Pool.GetConnected(*b.Bridge.LastActiveWorkspace)
		}
		if cdpClient == nil {
			return cb.AnswerCallbackQuery(ctx, "Not connected.", false)
		}

		if b.PromptDispatcher != nil {
			var th *int
			if ch.ThreadID != 0 {
				t := ch.ThreadID
				th = &t
			}
			go func() {
				_ = b.PromptDispatcher.Send(ctx, bridge.PromptDispatchRequest{
					Channel: bridge.TelegramChannel{ChatID: ch.ChatID, ThreadID: th},
					Prompt:  template.Prompt,
					Cdp:     &cdpServiceWithContextsAdapter{client: cdpClient},
					Options: &bridge.PromptDispatchOptions{
						ChatSessionService: b.ChatSessionService,
						ChatSessionRepo:    b.ChatSessionRepo,
						TopicManager:       b.TopicManager,
						TitleGenerator:     b.TitleGenerator,
					},
				})
			}()
		}
		return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("Running: %s", template.Name), false)
	}

	// 8. Session selection: session_select:<title>
	if ui.IsSessionSelectID(data) {
		selectedTitle := strings.TrimPrefix(data, ui.SessionSelectID+":")
		key := ChannelKey(ch)
		if b.WorkspaceBindingRepo == nil {
			return cb.AnswerCallbackQuery(ctx, "No project bound.", false)
		}
		binding, err := b.WorkspaceBindingRepo.FindByChannelId(key)
		if err != nil || binding == nil {
			return cb.AnswerCallbackQuery(ctx, "No project bound.", false)
		}
		if b.WorkspaceService == nil || b.Pool == nil {
			return cb.AnswerCallbackQuery(ctx, "Internal service unavailable.", false)
		}
		workspacePath, err := b.WorkspaceService.GetWorkspacePath(binding.WorkspacePath)
		if err != nil {
			return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("Error: %s", err.Error()), false)
		}
		client, err := b.Pool.GetOrConnect(workspacePath)
		if err != nil {
			return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("Error: %s", err.Error()), false)
		}
		if b.ChatSessionService == nil {
			return cb.AnswerCallbackQuery(ctx, "ChatSessionService unavailable.", false)
		}
		activateResult := b.ChatSessionService.ActivateSessionByTitle(client, selectedTitle)
		if activateResult.OK {
			text := fmt.Sprintf("<b>🔗 Joined Session</b>\n\n<b>%s</b>", ui.EscapeHTML(selectedTitle))
			_ = cb.EditMessageText(ctx, text, nil)
			return cb.AnswerCallbackQuery(ctx, "", false)
		}
		return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("Failed: %s", activateResult.Error), false)
	}

	// 9. Approval action buttons: approve_action, always_allow_action, deny_action
	if approvalAction := bridge.ParseApprovalCustomID(data); approvalAction != nil {
		projectName := ""
		if approvalAction.ProjectName != nil {
			projectName = *approvalAction.ProjectName
		} else if b.Bridge != nil && b.Bridge.LastActiveWorkspace != nil {
			projectName = *b.Bridge.LastActiveWorkspace
		}
		if b.Bridge == nil || b.Bridge.Pool == nil {
			return cb.AnswerCallbackQuery(ctx, "Approval detector not found.", false)
		}
		detector := b.Bridge.Pool.GetApprovalDetector(projectName)
		if detector == nil {
			return cb.AnswerCallbackQuery(ctx, "Approval detector not found.", false)
		}

		var success bool
		var actionLabel string
		switch approvalAction.Action {
		case "approve":
			ok, _ := detector.ApproveButton(ctx)
			success = ok
			actionLabel = "Allow"
		case "always_allow":
			ok, _ := detector.AlwaysAllowButton(ctx)
			success = ok
			actionLabel = "Allow Chat"
		default:
			ok, _ := detector.DenyButton(ctx)
			success = ok
			actionLabel = "Deny"
		}

		if success {
			_ = cb.RemoveInlineKeyboard(ctx)
			return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("%s executed.", actionLabel), false)
		}
		return cb.AnswerCallbackQuery(ctx, "Button not found.", false)
	}

	// 10. Planning buttons (legacy format & new format)
	if planningAction := bridge.ParsePlanningCustomID(data); planningAction != nil {
		projectName := ""
		if planningAction.ProjectName != nil {
			projectName = *planningAction.ProjectName
		} else if b.Bridge != nil && b.Bridge.LastActiveWorkspace != nil {
			projectName = *b.Bridge.LastActiveWorkspace
		}
		if b.Bridge == nil || b.Bridge.Pool == nil {
			return cb.AnswerCallbackQuery(ctx, "Planning detector not found.", false)
		}
		detector := b.Bridge.Pool.GetPlanningDetector(projectName)
		if detector == nil {
			return cb.AnswerCallbackQuery(ctx, "Planning detector not found.", false)
		}

		if planningAction.Action == "open" {
			ok, _ := detector.ClickOpenButton(ctx)
			if ok {
				time.Sleep(500 * time.Millisecond)
				var planContent string
				for attempt := 0; attempt < 3; attempt++ {
					extractRes, _ := detector.ExtractPlanContent(ctx)
					if extractRes != nil && *extractRes != "" {
						planContent = *extractRes
						break
					}
					time.Sleep(500 * time.Millisecond)
				}
				if planContent != "" {
					chKey := ChannelKey(ch)
					pages := ui.PaginatePlanContent(planContent, ui.DefaultPageSize)
					b.SetPlanContentCache(chKey, pages)
					targetChannelStr := fmt.Sprintf("%d", ch.ChatID)
					if ch.ThreadID != 0 {
						targetChannelStr = fmt.Sprintf("%d", ch.ThreadID)
					}
					payload := ui.BuildPlanContentUI(pages, 0, projectName, targetChannelStr)
					_ = b.ReplyHTML(ctx, ch, payload.Text, payload.Keyboard)
				}
				return cb.AnswerCallbackQuery(ctx, "Opened", false)
			}
			return cb.AnswerCallbackQuery(ctx, "Open button not found.", false)
		}

		ok, _ := detector.ClickProceedButton(ctx)
		if ok {
			_ = cb.RemoveInlineKeyboard(ctx)
			return cb.AnswerCallbackQuery(ctx, "Proceeding...", false)
		}
		return cb.AnswerCallbackQuery(ctx, "Proceed button not found.", false)
	}

	// 10b. New plan UI buttons
	if strings.HasPrefix(data, ui.PlanViewBtn+":") {
		suffix := strings.TrimPrefix(data, ui.PlanViewBtn+":")
		parts := strings.Split(suffix, ":")
		projectName := parts[0]
		if b.Bridge == nil || b.Bridge.Pool == nil {
			return cb.AnswerCallbackQuery(ctx, "Planning detector not found.", false)
		}
		detector := b.Bridge.Pool.GetPlanningDetector(projectName)
		if detector == nil {
			return cb.AnswerCallbackQuery(ctx, "Planning detector not found.", false)
		}
		ok, _ := detector.ClickOpenButton(ctx)
		if ok {
			time.Sleep(500 * time.Millisecond)
			var planContent string
			for attempt := 0; attempt < 3; attempt++ {
				extractRes, _ := detector.ExtractPlanContent(ctx)
				if extractRes != nil && *extractRes != "" {
					planContent = *extractRes
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			if planContent != "" {
				chKey := ChannelKey(ch)
				pages := ui.PaginatePlanContent(planContent, ui.DefaultPageSize)
				b.SetPlanContentCache(chKey, pages)
				targetChannelStr := fmt.Sprintf("%d", ch.ChatID)
				if ch.ThreadID != 0 {
					targetChannelStr = fmt.Sprintf("%d", ch.ThreadID)
				}
				payload := ui.BuildPlanContentUI(pages, 0, projectName, targetChannelStr)
				_ = b.ReplyHTML(ctx, ch, payload.Text, payload.Keyboard)
			}
			return cb.AnswerCallbackQuery(ctx, "Opened", false)
		}
		return cb.AnswerCallbackQuery(ctx, "Open button not found.", false)
	}

	if strings.HasPrefix(data, ui.PlanProceedBtn+":") {
		suffix := strings.TrimPrefix(data, ui.PlanProceedBtn+":")
		parts := strings.Split(suffix, ":")
		projectName := parts[0]
		if b.Bridge == nil || b.Bridge.Pool == nil {
			return cb.AnswerCallbackQuery(ctx, "Planning detector not found.", false)
		}
		detector := b.Bridge.Pool.GetPlanningDetector(projectName)
		if detector == nil {
			return cb.AnswerCallbackQuery(ctx, "Planning detector not found.", false)
		}
		ok, _ := detector.ClickProceedButton(ctx)
		if ok {
			b.ClearPlanEditPending(ChannelKey(ch))
			_ = cb.RemoveInlineKeyboard(ctx)
			return cb.AnswerCallbackQuery(ctx, "Proceeding...", false)
		}
		return cb.AnswerCallbackQuery(ctx, "Proceed button not found.", false)
	}

	if strings.HasPrefix(data, ui.PlanEditBtn+":") {
		suffix := strings.TrimPrefix(data, ui.PlanEditBtn+":")
		parts := strings.Split(suffix, ":")
		projectName := parts[0]
		b.SetPlanEditPending(ChannelKey(ch), projectName)
		_ = cb.AnswerCallbackQuery(ctx, "Type your edit instructions (or /cancel).", false)
		return b.ReplyHTML(ctx, ch, "<b>Edit Plan</b>\n\nType your plan edit instructions below.\nSend <code>/cancel</code> to cancel.", nil)
	}

	if strings.HasPrefix(data, ui.PlanRefreshBtn+":") {
		suffix := strings.TrimPrefix(data, ui.PlanRefreshBtn+":")
		parts := strings.Split(suffix, ":")
		projectName := parts[0]
		targetChannelStr := fmt.Sprintf("%d", ch.ChatID)
		if len(parts) > 1 && parts[1] != "" {
			targetChannelStr = parts[1]
		}
		if b.Bridge == nil || b.Bridge.Pool == nil {
			return cb.AnswerCallbackQuery(ctx, "Planning detector not found.", false)
		}
		detector := b.Bridge.Pool.GetPlanningDetector(projectName)
		if detector == nil {
			return cb.AnswerCallbackQuery(ctx, "Planning detector not found.", false)
		}
		rawInfo := detector.GetLastDetectedInfo()
		if rawInfo != nil {
			planningInfo := ui.PlanningInfo{
				OpenText:    rawInfo.OpenText,
				ProceedText: rawInfo.ProceedText,
				PlanTitle:   rawInfo.PlanTitle,
				PlanSummary: rawInfo.PlanSummary,
				Description: rawInfo.Description,
			}
			payload := ui.BuildPlanNotificationUI(planningInfo, projectName, targetChannelStr)
			_ = cb.EditMessageText(ctx, payload.Text, payload.Keyboard)
		}
		return cb.AnswerCallbackQuery(ctx, "Refreshed", false)
	}

	if strings.HasPrefix(data, ui.PlanPagePrefix+":") {
		rest := strings.TrimPrefix(data, ui.PlanPagePrefix+":")
		colonIdx := strings.Index(rest, ":")
		if colonIdx == -1 {
			return cb.AnswerCallbackQuery(ctx, "Page not found.", false)
		}
		page, err := strconv.Atoi(rest[:colonIdx])
		if err != nil {
			return cb.AnswerCallbackQuery(ctx, "Page not found.", false)
		}
		suffix := rest[colonIdx+1:]
		parts := strings.Split(suffix, ":")
		projectName := parts[0]
		targetChannelStr := fmt.Sprintf("%d", ch.ChatID)
		if len(parts) > 1 && parts[1] != "" {
			targetChannelStr = parts[1]
		}
		chKey := ChannelKey(ch)
		pages, ok := b.GetPlanContentCache(chKey)
		if !ok || page < 0 || page >= len(pages) {
			return cb.AnswerCallbackQuery(ctx, "Page not found.", false)
		}
		payload := ui.BuildPlanContentUI(pages, page, projectName, targetChannelStr)
		_ = cb.EditMessageText(ctx, payload.Text, payload.Keyboard)
		return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("Page %d/%d", page+1, len(pages)), false)
	}

	// 11. Error popup action buttons
	if errorAction := bridge.ParseErrorPopupCustomID(data); errorAction != nil {
		projectName := ""
		if errorAction.ProjectName != nil {
			projectName = *errorAction.ProjectName
		} else if b.Bridge != nil && b.Bridge.LastActiveWorkspace != nil {
			projectName = *b.Bridge.LastActiveWorkspace
		}
		if b.Bridge == nil || b.Bridge.Pool == nil {
			return cb.AnswerCallbackQuery(ctx, "Error popup detector not found.", false)
		}
		detector := b.Bridge.Pool.GetErrorPopupDetector(projectName)
		if detector == nil {
			return cb.AnswerCallbackQuery(ctx, "Error popup detector not found.", false)
		}

		switch errorAction.Action {
		case "dismiss":
			ok, _ := detector.ClickDismissButton(ctx)
			if ok {
				_ = cb.RemoveInlineKeyboard(ctx)
				return cb.AnswerCallbackQuery(ctx, "Dismissed", false)
			}
			return cb.AnswerCallbackQuery(ctx, "Button not found.", false)

		case "copy_debug":
			ok, _ := detector.ClickCopyDebugInfoButton(ctx)
			var clipboardContent string
			if ok {
				time.Sleep(300 * time.Millisecond)
				clipRes, _ := detector.ReadClipboard(ctx)
				if clipRes != nil && *clipRes != "" {
					clipboardContent = *clipRes
				}
				if clipboardContent != "" {
					truncated := clipboardContent
					if len(truncated) > 3800 {
						truncated = truncated[:3800] + "\n(truncated)"
					}
					debugText := fmt.Sprintf("<b>Debug Info</b>\n\n<pre>%s</pre>", ui.EscapeHTML(truncated))
					_ = b.ReplyHTML(ctx, ch, debugText, nil)
				}
			}
			feedback := "Button not found."
			if ok {
				if clipboardContent != "" {
					feedback = "Copied"
				} else {
					feedback = "Could not read clipboard."
				}
			}
			return cb.AnswerCallbackQuery(ctx, feedback, false)

		case "retry":
			ok, _ := detector.ClickRetryButton(ctx)
			if ok {
				_ = cb.RemoveInlineKeyboard(ctx)
				return cb.AnswerCallbackQuery(ctx, "Retrying...", false)
			}
			return cb.AnswerCallbackQuery(ctx, "Button not found.", false)
		}
	}

	// 12. Cleanup actions: cleanup_archive:<days>, cleanup_delete:<days>, cleanup_cancel
	if strings.HasPrefix(data, CleanupArchiveBtn) || strings.HasPrefix(data, CleanupDeleteBtn) || data == CleanupCancelBtn {
		if data == CleanupCancelBtn {
			_ = cb.EditMessagePlainText(ctx, "Cleanup cancelled.")
			return cb.AnswerCallbackQuery(ctx, "Cancelled", false)
		}

		isDelete := strings.HasPrefix(data, CleanupDeleteBtn)
		callbackDays := 7
		parts := strings.Split(data, ":")
		if len(parts) == 2 {
			if d, err := strconv.Atoi(parts[1]); err == nil && d > 0 {
				callbackDays = d
			}
		}

		guildID := fmt.Sprintf("%d", ch.ChatID)
		var inactive []InactiveSession
		if b.CleanupHandler != nil {
			inactive, _ = b.CleanupHandler.FindInactiveSessions(guildID, callbackDays)
		}

		sender := b.getSender()
		processed := 0
		for _, item := range inactive {
			if strings.Contains(item.Binding.ChannelID, ":") {
				channelParts := strings.Split(item.Binding.ChannelID, ":")
				if len(channelParts) == 2 {
					if threadID, err := strconv.Atoi(channelParts[1]); err == nil && threadID != 0 && sender != nil {
						if isDelete {
							_, _ = sender.DeleteForumTopic(ctx, &tgbot.DeleteForumTopicParams{
								ChatID:          ch.ChatID,
								MessageThreadID: threadID,
							})
						} else {
							_, _ = sender.CloseForumTopic(ctx, &tgbot.CloseForumTopicParams{
								ChatID:          ch.ChatID,
								MessageThreadID: threadID,
							})
						}
					}
				}
			}
			if b.CleanupHandler != nil {
				_ = b.CleanupHandler.CleanupByChannelID(item.Binding.ChannelID)
			}
			processed++
		}

		action := "archived"
		if isDelete {
			action = "deleted"
		}
		_ = cb.EditMessagePlainText(ctx, fmt.Sprintf("✅ Cleanup complete — %d session(s) %s.", processed, action))
		return cb.AnswerCallbackQuery(ctx, fmt.Sprintf("%d session(s) %s", processed, action), false)
	}

	return cb.AnswerCallbackQuery(ctx, "", false)
}
