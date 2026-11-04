package scheduler

import (
	"fmt"
	"sync"

	"antigravity-telegram-remote/internal/database"
	"github.com/robfig/cron/v3"
)

// JobCallback is the function executed when a cron schedule triggers.
type JobCallback func(schedule database.ScheduleRecord)

// ScheduledTask represents an active scheduled job that can be stopped or started.
type ScheduledTask interface {
	Stop()
	Start()
}

// CronEngine abstracts the cron scheduling engine for pluggability and unit testing.
type CronEngine interface {
	Validate(cronExpression string) bool
	Schedule(cronExpression string, fn func()) (ScheduledTask, error)
}

// ScheduleRepositoryInterface defines the repository contract required by ScheduleService.
type ScheduleRepositoryInterface interface {
	Create(input database.CreateScheduleInput) (database.ScheduleRecord, error)
	FindAll() ([]database.ScheduleRecord, error)
	FindById(id int64) (*database.ScheduleRecord, error)
	FindEnabled() ([]database.ScheduleRecord, error)
	Delete(id int64) (bool, error)
	Update(id int64, input database.UpdateScheduleInput) (bool, error)
}

type robfigTask struct {
	cron    *cron.Cron
	entryID cron.EntryID
}

func (t *robfigTask) Stop() {
	if t.cron != nil {
		t.cron.Remove(t.entryID)
	}
}

func (t *robfigTask) Start() {
	// In robfig/cron/v3, jobs are active once added while cron is running.
}

// RobfigCronEngine implements CronEngine using robfig/cron/v3.
type RobfigCronEngine struct {
	cron   *cron.Cron
	parser cron.Parser
}

// NewRobfigCronEngine creates and starts a new robfig/cron/v3 engine supporting standard 5-field and optional seconds.
func NewRobfigCronEngine() *RobfigCronEngine {
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	c := cron.New(cron.WithParser(parser))
	c.Start()
	return &RobfigCronEngine{
		cron:   c,
		parser: parser,
	}
}

// Validate checks whether the cron expression is valid.
func (e *RobfigCronEngine) Validate(cronExpression string) bool {
	_, err := e.parser.Parse(cronExpression)
	return err == nil
}

// Schedule registers a function to be executed on the given cron schedule.
func (e *RobfigCronEngine) Schedule(cronExpression string, fn func()) (ScheduledTask, error) {
	entryID, err := e.cron.AddFunc(cronExpression, fn)
	if err != nil {
		return nil, err
	}
	return &robfigTask{cron: e.cron, entryID: entryID}, nil
}

// Stop stops the underlying cron engine.
func (e *RobfigCronEngine) Stop() {
	if e.cron != nil {
		e.cron.Stop()
	}
}

// ScheduleService manages the lifecycle of scheduled jobs.
type ScheduleService struct {
	mu          sync.Mutex
	repo        ScheduleRepositoryInterface
	engine      CronEngine
	activeTasks map[int64]ScheduledTask
}

// NewScheduleService initializes a new ScheduleService.
func NewScheduleService(repo ScheduleRepositoryInterface, engine ...CronEngine) *ScheduleService {
	var eng CronEngine
	if len(engine) > 0 && engine[0] != nil {
		eng = engine[0]
	} else {
		eng = NewRobfigCronEngine()
	}

	return &ScheduleService{
		repo:        repo,
		engine:      eng,
		activeTasks: make(map[int64]ScheduledTask),
	}
}

// RestoreAll loads all enabled schedules from the repository and registers them with the cron engine.
func (s *ScheduleService) RestoreAll(jobCallback JobCallback) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	enabledSchedules, err := s.repo.FindEnabled()
	if err != nil {
		return 0, err
	}

	for _, schedule := range enabledSchedules {
		s.registerCronTask(schedule, jobCallback)
	}

	return len(enabledSchedules), nil
}

// AddSchedule validates the cron expression, persists to SQLite, and registers the cron task.
func (s *ScheduleService) AddSchedule(
	cronExpression string,
	prompt string,
	workspacePath string,
	jobCallback JobCallback,
) (database.ScheduleRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.engine.Validate(cronExpression) {
		return database.ScheduleRecord{}, fmt.Errorf("Invalid cron expression: %s", cronExpression)
	}

	record, err := s.repo.Create(database.CreateScheduleInput{
		CronExpression: cronExpression,
		Prompt:         prompt,
		WorkspacePath:  workspacePath,
		Enabled:        true,
	})
	if err != nil {
		return database.ScheduleRecord{}, err
	}

	s.registerCronTask(record, jobCallback)
	return record, nil
}

// RemoveSchedule stops the running cron job and deletes it from the database.
func (s *ScheduleService) RemoveSchedule(scheduleId int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, ok := s.activeTasks[scheduleId]; ok {
		task.Stop()
		delete(s.activeTasks, scheduleId)
	}

	return s.repo.Delete(scheduleId)
}

// StopAll stops all running cron jobs and clears the active tasks map.
func (s *ScheduleService) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range s.activeTasks {
		task.Stop()
	}
	s.activeTasks = make(map[int64]ScheduledTask)
}

// ListSchedules retrieves all schedules from the repository.
func (s *ScheduleService) ListSchedules() ([]database.ScheduleRecord, error) {
	return s.repo.FindAll()
}

func (s *ScheduleService) registerCronTask(schedule database.ScheduleRecord, jobCallback JobCallback) {
	rec := schedule
	task, err := s.engine.Schedule(rec.CronExpression, func() {
		jobCallback(rec)
	})
	if err == nil && task != nil {
		s.activeTasks[rec.ID] = task
	}
}
