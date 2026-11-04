package bridge

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func intPtr(i int) *int {
	return &i
}

func TestPromptDispatcher_InvokesSendPromptImpl(t *testing.T) {
	var calledWithBridge any
	var calledWithChannel TelegramChannel
	var calledWithPrompt string
	var calledWithCdp CdpEvaluator
	var calledWithMode *ModeService
	var calledWithModel *ModelService
	var calledWithImages []InboundImageAttachment
	var calledWithOptions *PromptDispatchOptions

	modeSvc := NewModeService()
	modelSvc := NewModelService()
	mockCdp := &mockCdpEvaluator{wsName: "TestProject"}

	sendImpl := func(
		ctx context.Context,
		b any,
		ch TelegramChannel,
		prompt string,
		cdp CdpEvaluator,
		mSvc *ModeService,
		modSvc *ModelService,
		images []InboundImageAttachment,
		opts *PromptDispatchOptions,
	) error {
		calledWithBridge = b
		calledWithChannel = ch
		calledWithPrompt = prompt
		calledWithCdp = cdp
		calledWithMode = mSvc
		calledWithModel = modSvc
		calledWithImages = images
		calledWithOptions = opts
		return nil
	}

	dispatcher := NewPromptDispatcher(PromptDispatcherDeps{
		Bridge:         "bridge-instance",
		ModeService:    modeSvc,
		ModelService:   modelSvc,
		SendPromptImpl: sendImpl,
	})

	opts := &PromptDispatchOptions{}
	req := PromptDispatchRequest{
		Channel:       TelegramChannel{ChatID: 12345, ThreadID: intPtr(1)},
		Prompt:        "hello",
		Cdp:           mockCdp,
		InboundImages: []InboundImageAttachment{},
		Options:       opts,
	}

	err := dispatcher.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calledWithBridge != "bridge-instance" {
		t.Fatalf("expected bridge 'bridge-instance', got %v", calledWithBridge)
	}
	if calledWithChannel.ChatID != 12345 || calledWithChannel.ThreadID == nil || *calledWithChannel.ThreadID != 1 {
		t.Fatalf("unexpected channel: %+v", calledWithChannel)
	}
	if calledWithPrompt != "hello" {
		t.Fatalf("expected prompt 'hello', got %q", calledWithPrompt)
	}
	if calledWithCdp != mockCdp {
		t.Fatalf("unexpected CDP instance")
	}
	if calledWithMode != modeSvc {
		t.Fatalf("unexpected ModeService")
	}
	if calledWithModel != modelSvc {
		t.Fatalf("unexpected ModelService")
	}
	if len(calledWithImages) != 0 {
		t.Fatalf("expected 0 images, got %d", len(calledWithImages))
	}
	if calledWithOptions != opts {
		t.Fatalf("unexpected Options")
	}
}

func TestPromptDispatcher_SerializesSendsToSameChannel(t *testing.T) {
	var mu sync.Mutex
	var order []int
	firstStarted := make(chan struct{})
	firstDone := make(chan struct{})

	sendImpl := func(
		ctx context.Context,
		b any,
		ch TelegramChannel,
		prompt string,
		cdp CdpEvaluator,
		mSvc *ModeService,
		modSvc *ModelService,
		images []InboundImageAttachment,
		opts *PromptDispatchOptions,
	) error {
		mu.Lock()
		if prompt == "first" {
			order = append(order, 1)
			mu.Unlock()
			close(firstStarted)
			<-firstDone
			return nil
		}
		order = append(order, 2)
		mu.Unlock()
		return nil
	}

	dispatcher := NewPromptDispatcher(PromptDispatcherDeps{
		SendPromptImpl: sendImpl,
	})

	req1 := PromptDispatchRequest{
		Channel: TelegramChannel{ChatID: 100, ThreadID: intPtr(1)},
		Prompt:  "first",
		Cdp:     &mockCdpEvaluator{wsName: "TestProject"},
	}
	req2 := PromptDispatchRequest{
		Channel: TelegramChannel{ChatID: 100, ThreadID: intPtr(1)},
		Prompt:  "second",
		Cdp:     &mockCdpEvaluator{wsName: "TestProject"},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = dispatcher.Send(context.Background(), req1)
	}()

	// Wait until first has started
	<-firstStarted

	go func() {
		defer wg.Done()
		_ = dispatcher.Send(context.Background(), req2)
	}()

	// Sleep briefly to ensure req2 is waiting
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	if !reflect.DeepEqual(order, []int{1}) {
		t.Fatalf("expected order [1] while first is running, got %v", order)
	}
	mu.Unlock()

	// Release first
	close(firstDone)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(order, []int{1, 2}) {
		t.Fatalf("expected order [1, 2], got %v", order)
	}
}

func TestPromptDispatcher_AllowsParallelSendsToDifferentChannelsAndWorkspaces(t *testing.T) {
	var mu sync.Mutex
	var order []string

	sendImpl := func(
		ctx context.Context,
		b any,
		ch TelegramChannel,
		prompt string,
		cdp CdpEvaluator,
		mSvc *ModeService,
		modSvc *ModelService,
		images []InboundImageAttachment,
		opts *PromptDispatchOptions,
	) error {
		mu.Lock()
		order = append(order, prompt)
		mu.Unlock()
		return nil
	}

	dispatcher := NewPromptDispatcher(PromptDispatcherDeps{
		SendPromptImpl: sendImpl,
	})

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = dispatcher.Send(context.Background(), PromptDispatchRequest{
			Channel: TelegramChannel{ChatID: 100, ThreadID: intPtr(1)},
			Prompt:  "channel-A",
			Cdp:     &mockCdpEvaluator{wsName: "WorkspaceA"},
		})
	}()

	go func() {
		defer wg.Done()
		_ = dispatcher.Send(context.Background(), PromptDispatchRequest{
			Channel: TelegramChannel{ChatID: 200, ThreadID: intPtr(2)},
			Prompt:  "channel-B",
			Cdp:     &mockCdpEvaluator{wsName: "WorkspaceB"},
		})
	}()

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(order))
	}
	hasA, hasB := false, false
	for _, p := range order {
		if p == "channel-A" {
			hasA = true
		}
		if p == "channel-B" {
			hasB = true
		}
	}
	if !hasA || !hasB {
		t.Fatalf("expected both channels to be processed, got %v", order)
	}
}

func TestPromptDispatcher_GeneratesDifferentKeysForDifferentThreadIDs(t *testing.T) {
	var mu sync.Mutex
	var calls []string

	sendImpl := func(
		ctx context.Context,
		b any,
		ch TelegramChannel,
		prompt string,
		cdp CdpEvaluator,
		mSvc *ModeService,
		modSvc *ModelService,
		images []InboundImageAttachment,
		opts *PromptDispatchOptions,
	) error {
		mu.Lock()
		calls = append(calls, prompt)
		mu.Unlock()
		return nil
	}

	dispatcher := NewPromptDispatcher(PromptDispatcherDeps{
		SendPromptImpl: sendImpl,
	})

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = dispatcher.Send(context.Background(), PromptDispatchRequest{
			Channel: TelegramChannel{ChatID: 100, ThreadID: intPtr(1)},
			Prompt:  "thread-1",
			Cdp:     &mockCdpEvaluator{wsName: "WorkspaceA"},
		})
	}()

	go func() {
		defer wg.Done()
		_ = dispatcher.Send(context.Background(), PromptDispatchRequest{
			Channel: TelegramChannel{ChatID: 100, ThreadID: intPtr(2)},
			Prompt:  "thread-2",
			Cdp:     &mockCdpEvaluator{wsName: "WorkspaceB"},
		})
	}()

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("expected 2 independent calls, got %d", len(calls))
	}
}

func TestPromptDispatcher_UsesChatIDOnlyWhenNoThreadID(t *testing.T) {
	called := 0
	sendImpl := func(
		ctx context.Context,
		b any,
		ch TelegramChannel,
		prompt string,
		cdp CdpEvaluator,
		mSvc *ModeService,
		modSvc *ModelService,
		images []InboundImageAttachment,
		opts *PromptDispatchOptions,
	) error {
		called++
		return nil
	}

	dispatcher := NewPromptDispatcher(PromptDispatcherDeps{
		SendPromptImpl: sendImpl,
	})

	err := dispatcher.Send(context.Background(), PromptDispatchRequest{
		Channel: TelegramChannel{ChatID: 100, ThreadID: nil},
		Prompt:  "no-thread",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected called 1 time, got %d", called)
	}
}

func TestPromptDispatcher_CleansUpLockAfterSendCompletes(t *testing.T) {
	called := 0
	sendImpl := func(
		ctx context.Context,
		b any,
		ch TelegramChannel,
		prompt string,
		cdp CdpEvaluator,
		mSvc *ModeService,
		modSvc *ModelService,
		images []InboundImageAttachment,
		opts *PromptDispatchOptions,
	) error {
		called++
		return nil
	}

	dispatcher := NewPromptDispatcher(PromptDispatcherDeps{
		SendPromptImpl: sendImpl,
	})

	req := PromptDispatchRequest{
		Channel: TelegramChannel{ChatID: 100, ThreadID: intPtr(1)},
		Prompt:  "p1",
		Cdp:     &mockCdpEvaluator{wsName: "TestProject"},
	}

	_ = dispatcher.Send(context.Background(), req)
	_ = dispatcher.Send(context.Background(), req)

	if called != 2 {
		t.Fatalf("expected 2 calls, got %d", called)
	}

	dispatcher.mu.Lock()
	wsCount := len(dispatcher.workspaceLocks)
	chCount := len(dispatcher.channelLocks)
	dispatcher.mu.Unlock()

	if wsCount != 0 {
		t.Fatalf("expected workspaceLocks to be empty after completion, got %d entries", wsCount)
	}
	if chCount != 0 {
		t.Fatalf("expected channelLocks to be empty after completion, got %d entries", chCount)
	}
}

func TestPromptDispatcher_HandlesErrorsWithoutBreakingSerialization(t *testing.T) {
	var mu sync.Mutex
	called := 0

	sendImpl := func(
		ctx context.Context,
		b any,
		ch TelegramChannel,
		prompt string,
		cdp CdpEvaluator,
		mSvc *ModeService,
		modSvc *ModelService,
		images []InboundImageAttachment,
		opts *PromptDispatchOptions,
	) error {
		mu.Lock()
		defer mu.Unlock()
		called++
		if prompt == "will-fail" {
			return errors.New("fail")
		}
		return nil
	}

	dispatcher := NewPromptDispatcher(PromptDispatcherDeps{
		SendPromptImpl: sendImpl,
	})

	req1 := PromptDispatchRequest{
		Channel: TelegramChannel{ChatID: 100, ThreadID: intPtr(1)},
		Prompt:  "will-fail",
		Cdp:     &mockCdpEvaluator{wsName: "TestProject"},
	}
	req2 := PromptDispatchRequest{
		Channel: TelegramChannel{ChatID: 100, ThreadID: intPtr(1)},
		Prompt:  "should-succeed",
		Cdp:     &mockCdpEvaluator{wsName: "TestProject"},
	}

	_ = dispatcher.Send(context.Background(), req1)
	_ = dispatcher.Send(context.Background(), req2)

	mu.Lock()
	defer mu.Unlock()
	if called != 2 {
		t.Fatalf("expected 2 calls, got %d", called)
	}
}
