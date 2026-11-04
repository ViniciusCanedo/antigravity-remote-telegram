package bridge

import (
	"context"
	"fmt"
	"sync"

	"antigravity-telegram-remote/internal/database"
)

// TelegramChannel represents a Telegram chat and optional thread (topic).
type TelegramChannel struct {
	ChatID   int64
	ThreadID *int
}

// InboundImageAttachment represents an incoming image attachment.
type InboundImageAttachment struct {
	FileName string
	MimeType string
	Base64   string
}

// PromptDispatchOptions contains optional context and repositories for dispatching.
type PromptDispatchOptions struct {
	ChatSessionService any
	ChatSessionRepo    *database.ChatSessionRepository
	TopicManager       any
	TitleGenerator     *TitleGeneratorService
}

// PromptDispatchRequest encapsulates all parameters required to send a user prompt.
type PromptDispatchRequest struct {
	Channel       TelegramChannel
	Prompt        string
	Cdp           CdpEvaluator
	InboundImages []InboundImageAttachment
	Options       *PromptDispatchOptions
}

// SendPromptFunc defines the function signature for executing a prompt.
type SendPromptFunc func(
	ctx context.Context,
	bridge any,
	channel TelegramChannel,
	prompt string,
	cdp CdpEvaluator,
	modeService *ModeService,
	modelService *ModelService,
	inboundImages []InboundImageAttachment,
	options *PromptDispatchOptions,
) error

// PromptDispatcherDeps contains the dependencies needed by PromptDispatcher.
type PromptDispatcherDeps struct {
	Bridge         any
	ModeService    *ModeService
	ModelService   *ModelService
	SendPromptImpl SendPromptFunc
}

type lockEntry struct {
	done chan struct{}
}

// PromptDispatcher serializes prompt dispatching per workspace and per channel.
type PromptDispatcher struct {
	mu             sync.Mutex
	deps           PromptDispatcherDeps
	channelLocks   map[string]*lockEntry
	workspaceLocks map[string]*lockEntry
}

// NewPromptDispatcher creates a new PromptDispatcher.
func NewPromptDispatcher(deps PromptDispatcherDeps) *PromptDispatcher {
	return &PromptDispatcher{
		deps:           deps,
		channelLocks:   make(map[string]*lockEntry),
		workspaceLocks: make(map[string]*lockEntry),
	}
}

func (d *PromptDispatcher) channelKey(ch TelegramChannel) string {
	if ch.ThreadID != nil && *ch.ThreadID != 0 {
		return fmt.Sprintf("%d:%d", ch.ChatID, *ch.ThreadID)
	}
	return fmt.Sprintf("%d", ch.ChatID)
}

// Send dispatches the prompt, serializing requests per workspace and per channel.
func (d *PromptDispatcher) Send(ctx context.Context, req PromptDispatchRequest) error {
	chKey := d.channelKey(req.Channel)
	var wsKey *string
	if req.Cdp != nil {
		wsName := req.Cdp.GetCurrentWorkspaceName()
		if wsName != "" {
			k := "ws:" + wsName
			wsKey = &k
		}
	}

	lockKey := chKey
	if wsKey != nil {
		lockKey = *wsKey
	}

	current := &lockEntry{done: make(chan struct{})}

	d.mu.Lock()
	prev := d.workspaceLocks[lockKey]
	d.workspaceLocks[lockKey] = current
	d.channelLocks[chKey] = current
	d.mu.Unlock()

	defer func() {
		close(current.done)
		d.mu.Lock()
		if d.workspaceLocks[lockKey] == current {
			delete(d.workspaceLocks, lockKey)
		}
		if d.channelLocks[chKey] == current {
			delete(d.channelLocks, chKey)
		}
		d.mu.Unlock()
	}()

	if prev != nil {
		select {
		case <-prev.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	inboundImages := req.InboundImages
	if inboundImages == nil {
		inboundImages = []InboundImageAttachment{}
	}

	if d.deps.SendPromptImpl != nil {
		_ = d.deps.SendPromptImpl(
			ctx,
			d.deps.Bridge,
			req.Channel,
			req.Prompt,
			req.Cdp,
			d.deps.ModeService,
			d.deps.ModelService,
			inboundImages,
			req.Options,
		)
	}

	return nil
}
