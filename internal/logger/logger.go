package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// LogLevel represents the logging severity levels supported by Logger.
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
	LevelNone  LogLevel = "none"
)

var levelPriority = map[LogLevel]int{
	LevelDebug: 0,
	LevelInfo:  1,
	LevelWarn:  2,
	LevelError: 3,
	LevelNone:  4,
}

// AnsiColors groups the exact ANSI escape sequences used for terminal formatting.
type AnsiColors struct {
	Red     string
	Yellow  string
	Cyan    string
	Green   string
	Magenta string
	Dim     string
	Reset   string
}

// COLORS provides standard ANSI terminal colors matching the TypeScript implementation.
var COLORS = AnsiColors{
	Red:     "\x1b[31m",
	Yellow:  "\x1b[33m",
	Cyan:    "\x1b[36m",
	Green:   "\x1b[32m",
	Magenta: "\x1b[35m",
	Dim:     "\x1b[2m",
	Reset:   "\x1b[0m",
}

// Logger defines the interface equivalent to Logger in src/utils/logger.ts.
type Logger interface {
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)
	Debug(args ...any)
	Phase(args ...any)
	Done(args ...any)
	Divider(label ...string)
	SetLogLevel(level LogLevel)
	GetLogLevel() LogLevel
}

// ConsoleWriters allows injecting custom output writers and clock in unit tests.
type ConsoleWriters struct {
	InfoOut  io.Writer
	WarnOut  io.Writer
	ErrorOut io.Writer
	DebugOut io.Writer
	NowFunc  func() time.Time
}

type standardLogger struct {
	mu           sync.RWMutex
	currentLevel LogLevel
	writers      ConsoleWriters
}

// CreateLogger creates a new Logger instance with an optional initial level (defaults to LevelInfo).
func CreateLogger(initialLevel ...LogLevel) Logger {
	lvl := LevelInfo
	if len(initialLevel) > 0 && initialLevel[0] != "" {
		lvl = initialLevel[0]
	}
	return CreateLoggerWithWriters(lvl, ConsoleWriters{
		InfoOut:  os.Stdout,
		WarnOut:  os.Stderr,
		ErrorOut: os.Stderr,
		DebugOut: os.Stdout,
		NowFunc:  time.Now,
	})
}

// CreateLoggerWithWriters creates a Logger instance with custom writers for testing.
func CreateLoggerWithWriters(initialLevel LogLevel, writers ConsoleWriters) Logger {
	if writers.InfoOut == nil {
		writers.InfoOut = os.Stdout
	}
	if writers.WarnOut == nil {
		writers.WarnOut = os.Stderr
	}
	if writers.ErrorOut == nil {
		writers.ErrorOut = os.Stderr
	}
	if writers.DebugOut == nil {
		writers.DebugOut = os.Stdout
	}
	if writers.NowFunc == nil {
		writers.NowFunc = time.Now
	}

	return &standardLogger{
		currentLevel: initialLevel,
		writers:      writers,
	}
}

// Default is the global singleton logger equivalent to `export const logger = createLogger('info')`.
var Default Logger = CreateLogger(LevelInfo)

func (l *standardLogger) shouldLog(methodLevel LogLevel) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return levelPriority[methodLevel] >= levelPriority[l.currentLevel]
}

func (l *standardLogger) getTimestamp() string {
	now := l.writers.NowFunc()
	timeString := now.Format("15:04:05")
	return fmt.Sprintf("%s[%s]%s", COLORS.Dim, timeString, COLORS.Reset)
}

func formatMessage(prefix string, args ...any) string {
	if len(args) == 0 {
		return prefix + "\n"
	}
	msg := fmt.Sprintln(args...)
	return prefix + " " + msg
}

func (l *standardLogger) Info(args ...any) {
	if !l.shouldLog(LevelInfo) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	prefix := fmt.Sprintf("%s %s[INFO]%s", l.getTimestamp(), COLORS.Cyan, COLORS.Reset)
	_, _ = fmt.Fprint(l.writers.InfoOut, formatMessage(prefix, args...))
}

func (l *standardLogger) Warn(args ...any) {
	if !l.shouldLog(LevelWarn) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	prefix := fmt.Sprintf("%s %s[WARN]%s", l.getTimestamp(), COLORS.Yellow, COLORS.Reset)
	_, _ = fmt.Fprint(l.writers.WarnOut, formatMessage(prefix, args...))
}

func (l *standardLogger) Error(args ...any) {
	if !l.shouldLog(LevelError) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	prefix := fmt.Sprintf("%s %s[ERROR]%s", l.getTimestamp(), COLORS.Red, COLORS.Reset)
	_, _ = fmt.Fprint(l.writers.ErrorOut, formatMessage(prefix, args...))
}

func (l *standardLogger) Debug(args ...any) {
	if !l.shouldLog(LevelDebug) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	prefix := fmt.Sprintf("%s %s[DEBUG]%s", l.getTimestamp(), COLORS.Dim, COLORS.Reset)
	_, _ = fmt.Fprint(l.writers.DebugOut, formatMessage(prefix, args...))
}

// Phase logs important state transitions (magenta).
func (l *standardLogger) Phase(args ...any) {
	if !l.shouldLog(LevelInfo) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	prefix := fmt.Sprintf("%s %s[PHASE]%s", l.getTimestamp(), COLORS.Magenta, COLORS.Reset)
	_, _ = fmt.Fprint(l.writers.InfoOut, formatMessage(prefix, args...))
}

// Done logs completion-related events (green).
func (l *standardLogger) Done(args ...any) {
	if !l.shouldLog(LevelInfo) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	prefix := fmt.Sprintf("%s %s[DONE]%s", l.getTimestamp(), COLORS.Green, COLORS.Reset)
	_, _ = fmt.Fprint(l.writers.InfoOut, formatMessage(prefix, args...))
}

// Divider outputs a section divider with optional label at info level.
func (l *standardLogger) Divider(label ...string) {
	if !l.shouldLog(LevelInfo) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	var lbl string
	if len(label) > 0 {
		lbl = label[0]
	}

	if lbl != "" {
		runeCount := utf8.RuneCountInString(lbl)
		pad := 50 - runeCount - 4
		if pad < 4 {
			pad = 4
		}
		_, _ = fmt.Fprintf(l.writers.InfoOut, "%s[DONE]%s %s── %s %s%s\n",
			COLORS.Green, COLORS.Reset, COLORS.Dim, lbl, strings.Repeat("─", pad), COLORS.Reset)
	} else {
		_, _ = fmt.Fprintf(l.writers.InfoOut, "%s[DONE]%s %s%s%s\n",
			COLORS.Green, COLORS.Reset, COLORS.Dim, strings.Repeat("─", 50), COLORS.Reset)
	}
}

func (l *standardLogger) SetLogLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.currentLevel = level
}

func (l *standardLogger) GetLogLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.currentLevel
}

// Convenience package-level delegates:
func Info(args ...any)           { Default.Info(args...) }
func Warn(args ...any)           { Default.Warn(args...) }
func Error(args ...any)          { Default.Error(args...) }
func Debug(args ...any)          { Default.Debug(args...) }
func Phase(args ...any)          { Default.Phase(args...) }
func Done(args ...any)           { Default.Done(args...) }
func Divider(label ...string)    { Default.Divider(label...) }
func SetLogLevel(level LogLevel) { Default.SetLogLevel(level) }
func GetLogLevel() LogLevel      { return Default.GetLogLevel() }

// IsValidLogLevel reports whether the provided string is a valid log level.
func IsValidLogLevel(raw string) bool {
	switch LogLevel(strings.ToLower(raw)) {
	case LevelDebug, LevelInfo, LevelWarn, LevelError, LevelNone:
		return true
	default:
		return false
	}
}
