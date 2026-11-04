package logger

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

type testSpy struct {
	infoBuf  *bytes.Buffer
	warnBuf  *bytes.Buffer
	errorBuf *bytes.Buffer
	debugBuf *bytes.Buffer
	logger   Logger
}

func newTestSpy(level ...LogLevel) *testSpy {
	infoBuf := &bytes.Buffer{}
	warnBuf := &bytes.Buffer{}
	errorBuf := &bytes.Buffer{}
	debugBuf := &bytes.Buffer{}

	lvl := LevelInfo
	if len(level) > 0 {
		lvl = level[0]
	}

	fixedTime := time.Date(2026, 11, 4, 12, 34, 56, 0, time.UTC)
	writers := ConsoleWriters{
		InfoOut:  infoBuf,
		WarnOut:  warnBuf,
		ErrorOut: errorBuf,
		DebugOut: debugBuf,
		NowFunc: func() time.Time {
			return fixedTime
		},
	}

	log := CreateLoggerWithWriters(lvl, writers)
	return &testSpy{
		infoBuf:  infoBuf,
		warnBuf:  warnBuf,
		errorBuf: errorBuf,
		debugBuf: debugBuf,
		logger:   log,
	}
}

func lineCount(buf *bytes.Buffer) int {
	str := buf.String()
	if str == "" {
		return 0
	}
	return strings.Count(str, "\n")
}

func TestLogger_DefaultLevelInfo(t *testing.T) {
	spy := newTestSpy(LevelInfo)

	spy.logger.Info("info message")
	spy.logger.Warn("warn message")
	spy.logger.Error("error message")
	spy.logger.Debug("debug message")

	if lineCount(spy.infoBuf) != 1 {
		t.Errorf("expected 1 info call, got %d", lineCount(spy.infoBuf))
	}
	if lineCount(spy.warnBuf) != 1 {
		t.Errorf("expected 1 warn call, got %d", lineCount(spy.warnBuf))
	}
	if lineCount(spy.errorBuf) != 1 {
		t.Errorf("expected 1 error call, got %d", lineCount(spy.errorBuf))
	}
	if lineCount(spy.debugBuf) != 0 {
		t.Errorf("expected 0 debug calls, got %d", lineCount(spy.debugBuf))
	}
}

func TestLogger_DebugLevel(t *testing.T) {
	spy := newTestSpy(LevelDebug)

	spy.logger.Debug("debug message")
	spy.logger.Info("info message")
	spy.logger.Warn("warn message")
	spy.logger.Error("error message")

	if lineCount(spy.debugBuf) != 1 {
		t.Errorf("expected 1 debug call, got %d", lineCount(spy.debugBuf))
	}
	if lineCount(spy.infoBuf) != 1 {
		t.Errorf("expected 1 info call, got %d", lineCount(spy.infoBuf))
	}
	if lineCount(spy.warnBuf) != 1 {
		t.Errorf("expected 1 warn call, got %d", lineCount(spy.warnBuf))
	}
	if lineCount(spy.errorBuf) != 1 {
		t.Errorf("expected 1 error call, got %d", lineCount(spy.errorBuf))
	}
}

func TestLogger_WarnLevel(t *testing.T) {
	spy := newTestSpy(LevelWarn)

	spy.logger.Debug("debug message")
	spy.logger.Info("info message")
	spy.logger.Warn("warn message")
	spy.logger.Error("error message")

	if lineCount(spy.debugBuf) != 0 {
		t.Errorf("expected 0 debug calls, got %d", lineCount(spy.debugBuf))
	}
	if lineCount(spy.infoBuf) != 0 {
		t.Errorf("expected 0 info calls, got %d", lineCount(spy.infoBuf))
	}
	if lineCount(spy.warnBuf) != 1 {
		t.Errorf("expected 1 warn call, got %d", lineCount(spy.warnBuf))
	}
	if lineCount(spy.errorBuf) != 1 {
		t.Errorf("expected 1 error call, got %d", lineCount(spy.errorBuf))
	}
}

func TestLogger_ErrorLevel(t *testing.T) {
	spy := newTestSpy(LevelError)

	spy.logger.Debug("debug message")
	spy.logger.Info("info message")
	spy.logger.Warn("warn message")
	spy.logger.Error("error message")

	if lineCount(spy.debugBuf) != 0 {
		t.Errorf("expected 0 debug calls, got %d", lineCount(spy.debugBuf))
	}
	if lineCount(spy.infoBuf) != 0 {
		t.Errorf("expected 0 info calls, got %d", lineCount(spy.infoBuf))
	}
	if lineCount(spy.warnBuf) != 0 {
		t.Errorf("expected 0 warn calls, got %d", lineCount(spy.warnBuf))
	}
	if lineCount(spy.errorBuf) != 1 {
		t.Errorf("expected 1 error call, got %d", lineCount(spy.errorBuf))
	}
}

func TestLogger_NoneLevel(t *testing.T) {
	spy := newTestSpy(LevelNone)

	spy.logger.Debug("debug message")
	spy.logger.Info("info message")
	spy.logger.Warn("warn message")
	spy.logger.Error("error message")
	spy.logger.Phase("phase message")
	spy.logger.Done("done message")
	spy.logger.Divider("section")

	if lineCount(spy.debugBuf) != 0 || lineCount(spy.infoBuf) != 0 ||
		lineCount(spy.warnBuf) != 0 || lineCount(spy.errorBuf) != 0 {
		t.Errorf("expected all writers empty, got debug=%d, info=%d, warn=%d, error=%d",
			lineCount(spy.debugBuf), lineCount(spy.infoBuf), lineCount(spy.warnBuf), lineCount(spy.errorBuf))
	}
}

func TestLogger_SetLogLevel_RuntimeChange(t *testing.T) {
	spy := newTestSpy(LevelInfo)

	spy.logger.Debug("should not appear")
	if lineCount(spy.debugBuf) != 0 {
		t.Errorf("expected 0 debug calls, got %d", lineCount(spy.debugBuf))
	}

	spy.logger.SetLogLevel(LevelDebug)

	spy.logger.Debug("should appear")
	if lineCount(spy.debugBuf) != 1 {
		t.Errorf("expected 1 debug call, got %d", lineCount(spy.debugBuf))
	}
}

func TestLogger_SetLogLevel_EscalateDebugToError(t *testing.T) {
	spy := newTestSpy(LevelDebug)

	spy.logger.Info("should appear")
	if lineCount(spy.infoBuf) != 1 {
		t.Errorf("expected 1 info call, got %d", lineCount(spy.infoBuf))
	}

	spy.logger.SetLogLevel(LevelError)

	spy.logger.Info("should not appear")
	if lineCount(spy.infoBuf) != 1 {
		t.Errorf("expected still 1 info call, got %d", lineCount(spy.infoBuf))
	}
}

func TestLogger_GetLogLevel(t *testing.T) {
	spy := newTestSpy(LevelWarn)
	if spy.logger.GetLogLevel() != LevelWarn {
		t.Errorf("expected warn, got %s", spy.logger.GetLogLevel())
	}

	spy.logger.SetLogLevel(LevelDebug)
	if spy.logger.GetLogLevel() != LevelDebug {
		t.Errorf("expected debug, got %s", spy.logger.GetLogLevel())
	}
}

func TestLogger_Helpers_PhaseAtInfo(t *testing.T) {
	spy := newTestSpy(LevelInfo)
	spy.logger.Phase("starting phase")
	if lineCount(spy.infoBuf) != 1 {
		t.Errorf("expected 1 info call, got %d", lineCount(spy.infoBuf))
	}
}

func TestLogger_Helpers_PhaseSuppressedAtWarn(t *testing.T) {
	spy := newTestSpy(LevelWarn)
	spy.logger.Phase("starting phase")
	if lineCount(spy.infoBuf) != 0 {
		t.Errorf("expected 0 info calls, got %d", lineCount(spy.infoBuf))
	}
}

func TestLogger_Helpers_DoneAtInfo(t *testing.T) {
	spy := newTestSpy(LevelInfo)
	spy.logger.Done("completed")
	if lineCount(spy.infoBuf) != 1 {
		t.Errorf("expected 1 info call, got %d", lineCount(spy.infoBuf))
	}
}

func TestLogger_Helpers_DoneSuppressedAtWarn(t *testing.T) {
	spy := newTestSpy(LevelWarn)
	spy.logger.Done("completed")
	if lineCount(spy.infoBuf) != 0 {
		t.Errorf("expected 0 info calls, got %d", lineCount(spy.infoBuf))
	}
}

func TestLogger_Helpers_DividerWithLabelAtInfo(t *testing.T) {
	spy := newTestSpy(LevelInfo)
	spy.logger.Divider("Section")
	if lineCount(spy.infoBuf) != 1 {
		t.Errorf("expected 1 info call, got %d", lineCount(spy.infoBuf))
	}
	out := spy.infoBuf.String()
	if !strings.Contains(out, "[DONE]") || !strings.Contains(out, "── Section ") {
		t.Errorf("unexpected divider output: %q", out)
	}
}

func TestLogger_Helpers_DividerWithoutLabelAtInfo(t *testing.T) {
	spy := newTestSpy(LevelInfo)
	spy.logger.Divider()
	if lineCount(spy.infoBuf) != 1 {
		t.Errorf("expected 1 info call, got %d", lineCount(spy.infoBuf))
	}
	out := spy.infoBuf.String()
	if !strings.Contains(out, "[DONE]") || !strings.Contains(out, strings.Repeat("─", 50)) {
		t.Errorf("unexpected divider output: %q", out)
	}
}

func TestLogger_Helpers_DividerSuppressedAtError(t *testing.T) {
	spy := newTestSpy(LevelError)
	spy.logger.Divider("Section")
	if lineCount(spy.infoBuf) != 0 {
		t.Errorf("expected 0 info calls, got %d", lineCount(spy.infoBuf))
	}
}

func TestLogger_Format_InfoTagAndColor(t *testing.T) {
	spy := newTestSpy(LevelInfo)
	spy.logger.Info("test message")

	output := spy.infoBuf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("expected output to contain [INFO], got %q", output)
	}
	if !strings.Contains(output, COLORS.Cyan) {
		t.Errorf("expected output to contain COLORS.Cyan, got %q", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got %q", output)
	}
}

func TestLogger_Format_WarnTagAndColor(t *testing.T) {
	spy := newTestSpy(LevelInfo)
	spy.logger.Warn("test message")

	output := spy.warnBuf.String()
	if !strings.Contains(output, "[WARN]") {
		t.Errorf("expected output to contain [WARN], got %q", output)
	}
	if !strings.Contains(output, COLORS.Yellow) {
		t.Errorf("expected output to contain COLORS.Yellow, got %q", output)
	}
}

func TestLogger_Format_ErrorTagAndColor(t *testing.T) {
	spy := newTestSpy(LevelInfo)
	spy.logger.Error("test message")

	output := spy.errorBuf.String()
	if !strings.Contains(output, "[ERROR]") {
		t.Errorf("expected output to contain [ERROR], got %q", output)
	}
	if !strings.Contains(output, COLORS.Red) {
		t.Errorf("expected output to contain COLORS.Red, got %q", output)
	}
}

func TestLogger_Format_DebugTagAndColor(t *testing.T) {
	spy := newTestSpy(LevelDebug)
	spy.logger.Debug("test message")

	output := spy.debugBuf.String()
	if !strings.Contains(output, "[DEBUG]") {
		t.Errorf("expected output to contain [DEBUG], got %q", output)
	}
	if !strings.Contains(output, COLORS.Dim) {
		t.Errorf("expected output to contain COLORS.Dim, got %q", output)
	}
}

func TestLogger_Format_PhaseTagAndColor(t *testing.T) {
	spy := newTestSpy(LevelInfo)
	spy.logger.Phase("test message")

	output := spy.infoBuf.String()
	if !strings.Contains(output, "[PHASE]") {
		t.Errorf("expected output to contain [PHASE], got %q", output)
	}
	if !strings.Contains(output, COLORS.Magenta) {
		t.Errorf("expected output to contain COLORS.Magenta, got %q", output)
	}
}

func TestLogger_Format_DoneTagAndColor(t *testing.T) {
	spy := newTestSpy(LevelInfo)
	spy.logger.Done("test message")

	output := spy.infoBuf.String()
	if !strings.Contains(output, "[DONE]") {
		t.Errorf("expected output to contain [DONE], got %q", output)
	}
	if !strings.Contains(output, COLORS.Green) {
		t.Errorf("expected output to contain COLORS.Green, got %q", output)
	}
}

func TestLogger_DefaultArgument(t *testing.T) {
	log := CreateLogger()
	if log.GetLogLevel() != LevelInfo {
		t.Errorf("expected info level by default, got %s", log.GetLogLevel())
	}
}

func TestLogger_IsValidLogLevel(t *testing.T) {
	valid := []string{"debug", "info", "warn", "error", "none", "DEBUG", "Info"}
	for _, v := range valid {
		if !IsValidLogLevel(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []string{"verbose", "trace", "INVALID", ""}
	for _, iv := range invalid {
		if IsValidLogLevel(iv) {
			t.Errorf("expected %q to be invalid", iv)
		}
	}
}
