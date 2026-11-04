package scheduler

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"antigravity-telegram-remote/internal/database"
)

type mockScheduledTask struct {
	mu          sync.Mutex
	stopCalled  bool
	startCalled bool
}

func (m *mockScheduledTask) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalled = true
}

func (m *mockScheduledTask) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalled = true
}

type mockCronEngine struct {
	mu            sync.Mutex
	validateFn    func(cronExpression string) bool
	scheduleFn    func(cronExpression string, fn func()) (ScheduledTask, error)
	validateCalls []string
	scheduleCalls []string
}

func (m *mockCronEngine) Validate(expr string) bool {
	m.mu.Lock()
	m.validateCalls = append(m.validateCalls, expr)
	fn := m.validateFn
	m.mu.Unlock()
	if fn != nil {
		return fn(expr)
	}
	return true
}

func (m *mockCronEngine) Schedule(expr string, fn func()) (ScheduledTask, error) {
	m.mu.Lock()
	m.scheduleCalls = append(m.scheduleCalls, expr)
	sFn := m.scheduleFn
	m.mu.Unlock()
	if sFn != nil {
		return sFn(expr, fn)
	}
	return &mockScheduledTask{}, nil
}

type mockScheduleRepo struct {
	mu               sync.Mutex
	createFn         func(input database.CreateScheduleInput) (database.ScheduleRecord, error)
	findAllFn        func() ([]database.ScheduleRecord, error)
	findByIdFn       func(id int64) (*database.ScheduleRecord, error)
	findEnabledFn    func() ([]database.ScheduleRecord, error)
	deleteFn         func(id int64) (bool, error)
	updateFn         func(id int64, input database.UpdateScheduleInput) (bool, error)
	createCalls      []database.CreateScheduleInput
	findAllCalls     int
	findEnabledCalls int
	deleteCalls      []int64
}

func (m *mockScheduleRepo) Create(input database.CreateScheduleInput) (database.ScheduleRecord, error) {
	m.mu.Lock()
	m.createCalls = append(m.createCalls, input)
	fn := m.createFn
	m.mu.Unlock()
	if fn != nil {
		return fn(input)
	}
	return database.ScheduleRecord{
		ID:             1,
		CronExpression: input.CronExpression,
		Prompt:         input.Prompt,
		WorkspacePath:  input.WorkspacePath,
		Enabled:        input.Enabled,
	}, nil
}

func (m *mockScheduleRepo) FindAll() ([]database.ScheduleRecord, error) {
	m.mu.Lock()
	m.findAllCalls++
	fn := m.findAllFn
	m.mu.Unlock()
	if fn != nil {
		return fn()
	}
	return nil, nil
}

func (m *mockScheduleRepo) FindById(id int64) (*database.ScheduleRecord, error) {
	m.mu.Lock()
	fn := m.findByIdFn
	m.mu.Unlock()
	if fn != nil {
		return fn(id)
	}
	return nil, nil
}

func (m *mockScheduleRepo) FindEnabled() ([]database.ScheduleRecord, error) {
	m.mu.Lock()
	m.findEnabledCalls++
	fn := m.findEnabledFn
	m.mu.Unlock()
	if fn != nil {
		return fn()
	}
	return nil, nil
}

func (m *mockScheduleRepo) Delete(id int64) (bool, error) {
	m.mu.Lock()
	m.deleteCalls = append(m.deleteCalls, id)
	fn := m.deleteFn
	m.mu.Unlock()
	if fn != nil {
		return fn(id)
	}
	return true, nil
}

func (m *mockScheduleRepo) Update(id int64, input database.UpdateScheduleInput) (bool, error) {
	m.mu.Lock()
	fn := m.updateFn
	m.mu.Unlock()
	if fn != nil {
		return fn(id, input)
	}
	return true, nil
}

func TestScheduleService_RestoreAll_RegistersAllEnabled(t *testing.T) {
	schedules := []database.ScheduleRecord{
		{
			ID:             1,
			CronExpression: "0 9 * * *",
			Prompt:         "朝のビルド",
			WorkspacePath:  "/project1",
			Enabled:        true,
		},
		{
			ID:             2,
			CronExpression: "0 18 * * *",
			Prompt:         "夕方のテスト",
			WorkspacePath:  "/project2",
			Enabled:        true,
		},
	}

	repo := &mockScheduleRepo{
		findEnabledFn: func() ([]database.ScheduleRecord, error) {
			return schedules, nil
		},
	}
	engine := &mockCronEngine{}
	service := NewScheduleService(repo, engine)

	jobCallbackCalls := 0
	jobCallback := func(rec database.ScheduleRecord) {
		jobCallbackCalls++
	}

	restoredCount, err := service.RestoreAll(jobCallback)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.findEnabledCalls != 1 {
		t.Fatalf("expected findEnabled called 1 time, got %d", repo.findEnabledCalls)
	}
	if len(engine.scheduleCalls) != 2 {
		t.Fatalf("expected schedule called 2 times, got %d", len(engine.scheduleCalls))
	}
	if engine.scheduleCalls[0] != "0 9 * * *" || engine.scheduleCalls[1] != "0 18 * * *" {
		t.Fatalf("unexpected schedule call expressions: %v", engine.scheduleCalls)
	}
	if restoredCount != 2 {
		t.Fatalf("expected restored count 2, got %d", restoredCount)
	}
}

func TestScheduleService_RestoreAll_ReturnsZeroWhenNoneEnabled(t *testing.T) {
	repo := &mockScheduleRepo{
		findEnabledFn: func() ([]database.ScheduleRecord, error) {
			return []database.ScheduleRecord{}, nil
		},
	}
	engine := &mockCronEngine{}
	service := NewScheduleService(repo, engine)

	restoredCount, err := service.RestoreAll(func(rec database.ScheduleRecord) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if restoredCount != 0 {
		t.Fatalf("expected restored count 0, got %d", restoredCount)
	}
	if len(engine.scheduleCalls) != 0 {
		t.Fatalf("expected 0 schedule calls, got %d", len(engine.scheduleCalls))
	}
}

func TestScheduleService_AddSchedule_ValidatesSavesAndSchedules(t *testing.T) {
	createdRecord := database.ScheduleRecord{
		ID:             1,
		CronExpression: "*/10 * * * *",
		Prompt:         "10分毎のチェック",
		WorkspacePath:  "/service",
		Enabled:        true,
	}

	repo := &mockScheduleRepo{
		createFn: func(input database.CreateScheduleInput) (database.ScheduleRecord, error) {
			return createdRecord, nil
		},
	}
	mockTask := &mockScheduledTask{}
	engine := &mockCronEngine{
		validateFn: func(cronExpression string) bool {
			return true
		},
		scheduleFn: func(cronExpression string, fn func()) (ScheduledTask, error) {
			return mockTask, nil
		},
	}

	service := NewScheduleService(repo, engine)
	record, err := service.AddSchedule(
		"*/10 * * * *",
		"10分毎のチェック",
		"/service",
		func(rec database.ScheduleRecord) {},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(engine.validateCalls) != 1 || engine.validateCalls[0] != "*/10 * * * *" {
		t.Fatalf("expected validate called with '*/10 * * * *', got %v", engine.validateCalls)
	}
	if len(repo.createCalls) != 1 {
		t.Fatalf("expected create called 1 time, got %d", len(repo.createCalls))
	}
	expectedInput := database.CreateScheduleInput{
		CronExpression: "*/10 * * * *",
		Prompt:         "10分毎のチェック",
		WorkspacePath:  "/service",
		Enabled:        true,
	}
	if repo.createCalls[0] != expectedInput {
		t.Fatalf("expected create input %v, got %v", expectedInput, repo.createCalls[0])
	}
	if len(engine.scheduleCalls) != 1 {
		t.Fatalf("expected schedule called 1 time, got %d", len(engine.scheduleCalls))
	}
	if !reflect.DeepEqual(record, createdRecord) {
		t.Fatalf("expected record %v, got %v", createdRecord, record)
	}
}

func TestScheduleService_AddSchedule_ThrowsOnInvalidCron(t *testing.T) {
	repo := &mockScheduleRepo{}
	engine := &mockCronEngine{
		validateFn: func(cronExpression string) bool {
			return false
		},
	}
	service := NewScheduleService(repo, engine)

	_, err := service.AddSchedule("invalid-cron", "テスト", "/test", func(rec database.ScheduleRecord) {})
	if err == nil {
		t.Fatal("expected error for invalid cron, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid cron expression: invalid-cron") {
		t.Fatalf("expected 'Invalid cron expression: invalid-cron', got %v", err)
	}
	if len(repo.createCalls) != 0 {
		t.Fatalf("expected repo.Create not called, got %d calls", len(repo.createCalls))
	}
	if len(engine.scheduleCalls) != 0 {
		t.Fatalf("expected engine.Schedule not called, got %d calls", len(engine.scheduleCalls))
	}
}

func TestScheduleService_RemoveSchedule_StopsTaskAndDeletesFromDB(t *testing.T) {
	mockTask := &mockScheduledTask{}
	repo := &mockScheduleRepo{
		createFn: func(input database.CreateScheduleInput) (database.ScheduleRecord, error) {
			return database.ScheduleRecord{
				ID:             5,
				CronExpression: "0 6 * * *",
				Prompt:         "朝のジョブ",
				WorkspacePath:  "/morning",
				Enabled:        true,
			}, nil
		},
		deleteFn: func(id int64) (bool, error) {
			return true, nil
		},
	}
	engine := &mockCronEngine{
		scheduleFn: func(cronExpression string, fn func()) (ScheduledTask, error) {
			return mockTask, nil
		},
	}

	service := NewScheduleService(repo, engine)
	_, _ = service.AddSchedule("0 6 * * *", "朝のジョブ", "/morning", func(rec database.ScheduleRecord) {})

	deleted, err := service.RemoveSchedule(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted to be true")
	}
	if !mockTask.stopCalled {
		t.Fatal("expected mockTask.Stop() to have been called")
	}
	if len(repo.deleteCalls) != 1 || repo.deleteCalls[0] != 5 {
		t.Fatalf("expected repo.Delete called with 5, got %v", repo.deleteCalls)
	}
}

func TestScheduleService_RemoveSchedule_ReturnsFalseForNonExistent(t *testing.T) {
	repo := &mockScheduleRepo{
		deleteFn: func(id int64) (bool, error) {
			return false, nil
		},
	}
	engine := &mockCronEngine{}
	service := NewScheduleService(repo, engine)

	deleted, err := service.RemoveSchedule(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted {
		t.Fatal("expected deleted to be false")
	}
}

func TestScheduleService_StopAll_StopsAllRunningJobs(t *testing.T) {
	mockTask1 := &mockScheduledTask{}
	mockTask2 := &mockScheduledTask{}
	tasks := []*mockScheduledTask{mockTask1, mockTask2}
	idx := 0

	repo := &mockScheduleRepo{
		createFn: func(input database.CreateScheduleInput) (database.ScheduleRecord, error) {
			id := int64(len(tasks) - idx)
			return database.ScheduleRecord{
				ID:             id,
				CronExpression: input.CronExpression,
				Prompt:         input.Prompt,
				WorkspacePath:  input.WorkspacePath,
				Enabled:        true,
			}, nil
		},
	}
	engine := &mockCronEngine{
		scheduleFn: func(cronExpression string, fn func()) (ScheduledTask, error) {
			t := tasks[idx]
			idx++
			return t, nil
		},
	}

	service := NewScheduleService(repo, engine)
	_, _ = service.AddSchedule("0 9 * * *", "テスト1", "/p1", func(rec database.ScheduleRecord) {})
	_, _ = service.AddSchedule("0 18 * * *", "テスト2", "/p2", func(rec database.ScheduleRecord) {})

	service.StopAll()

	if !mockTask1.stopCalled {
		t.Fatal("expected mockTask1 to be stopped")
	}
	if !mockTask2.stopCalled {
		t.Fatal("expected mockTask2 to be stopped")
	}
}

func TestScheduleService_ListSchedules_RetrievesAllFromDB(t *testing.T) {
	schedules := []database.ScheduleRecord{
		{ID: 1, CronExpression: "0 9 * * *", Prompt: "ジョブ1", WorkspacePath: "/p1", Enabled: true},
		{ID: 2, CronExpression: "0 18 * * *", Prompt: "ジョブ2", WorkspacePath: "/p2", Enabled: false},
	}
	repo := &mockScheduleRepo{
		findAllFn: func() ([]database.ScheduleRecord, error) {
			return schedules, nil
		},
	}
	service := NewScheduleService(repo, &mockCronEngine{})

	result, err := service.ListSchedules()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result, schedules) {
		t.Fatalf("expected %v, got %v", schedules, result)
	}
	if repo.findAllCalls != 1 {
		t.Fatalf("expected findAll called 1 time, got %d", repo.findAllCalls)
	}
}

func TestRobfigCronEngine_RealExecution(t *testing.T) {
	eng := NewRobfigCronEngine()
	defer eng.Stop()

	if !eng.Validate("*/5 * * * *") {
		t.Fatal("expected valid 5-field cron to pass validation")
	}
	if !eng.Validate("0 */5 * * * *") {
		t.Fatal("expected valid 6-field cron to pass validation")
	}
	if eng.Validate("not-a-cron") {
		t.Fatal("expected invalid cron to fail validation")
	}

	task, err := eng.Schedule("*/5 * * * *", func() {})
	if err != nil {
		t.Fatalf("unexpected error scheduling: %v", err)
	}
	task.Stop()
}
