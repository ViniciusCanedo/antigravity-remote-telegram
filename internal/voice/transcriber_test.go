package voice

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

type mockBotGetter struct {
	file *TelegramFileInfo
	err  error
}

func (m *mockBotGetter) GetFile(_ context.Context, _ string) (*TelegramFileInfo, error) {
	return m.file, m.err
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDownloadTelegramVoice_DownloadsToTempDir(t *testing.T) {
	botApi := &mockBotGetter{
		file: &TelegramFileInfo{FilePath: "voice/file_0.oga"},
	}
	botToken := "test-bot-token"
	voice := TelegramVoiceInfo{
		FileID:       "voice-file-id-123",
		FileUniqueID: "unique-456",
		Duration:     5,
		MimeType:     "audio/ogg",
	}

	mockHTTP := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			expectedURL := "https://api.telegram.org/file/bottest-bot-token/voice/file_0.oga"
			if req.URL.String() != expectedURL {
				t.Errorf("expected URL %s, got %s", expectedURL, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBuffer(make([]byte, 100))),
			}, nil
		}),
	}

	result, err := DownloadTelegramVoice(context.Background(), botApi, botToken, voice, mockHTTP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = os.Remove(result) }()

	matched, _ := regexp.MatchString(`unique-456\.oga$`, result)
	if !matched {
		t.Errorf("expected path to match unique-456.oga$, got %s", result)
	}

	stat, err := os.Stat(result)
	if err != nil {
		t.Fatalf("file does not exist on disk: %v", err)
	}
	if stat.Size() != 100 {
		t.Errorf("expected file size 100, got %d", stat.Size())
	}
}

func TestDownloadTelegramVoice_ErrorWhenNoFilePath(t *testing.T) {
	botApi := &mockBotGetter{
		file: &TelegramFileInfo{FilePath: ""},
	}
	botToken := "test-bot-token"
	voice := TelegramVoiceInfo{
		FileID:       "voice-file-id-123",
		FileUniqueID: "unique-456",
	}

	_, err := DownloadTelegramVoice(context.Background(), botApi, botToken, voice)
	if err == nil || !regexp.MustCompile(`Telegram returned no file_path`).MatchString(err.Error()) {
		t.Fatalf("expected error containing 'Telegram returned no file_path', got %v", err)
	}
}

func TestDownloadTelegramVoice_ErrorWhenHTTPNotOK(t *testing.T) {
	botApi := &mockBotGetter{
		file: &TelegramFileInfo{FilePath: "voice/file_0.oga"},
	}
	botToken := "test-bot-token"
	voice := TelegramVoiceInfo{
		FileID:       "voice-file-id-123",
		FileUniqueID: "unique-456",
	}

	mockHTTP := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString("Not Found")),
			}, nil
		}),
	}

	_, err := DownloadTelegramVoice(context.Background(), botApi, botToken, voice, mockHTTP)
	if err == nil || !regexp.MustCompile(`Voice download failed \(status=404\)`).MatchString(err.Error()) {
		t.Fatalf("expected error containing 'Voice download failed (status=404)', got %v", err)
	}
}

func TestDownloadTelegramVoice_ErrorWhenFileEmpty(t *testing.T) {
	botApi := &mockBotGetter{
		file: &TelegramFileInfo{FilePath: "voice/file_0.oga"},
	}
	botToken := "test-bot-token"
	voice := TelegramVoiceInfo{
		FileID:       "voice-file-id-123",
		FileUniqueID: "unique-456",
	}

	mockHTTP := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBuffer([]byte{})),
			}, nil
		}),
	}

	_, err := DownloadTelegramVoice(context.Background(), botApi, botToken, voice, mockHTTP)
	if err == nil || !regexp.MustCompile(`Voice download returned empty file`).MatchString(err.Error()) {
		t.Fatalf("expected error containing 'Voice download returned empty file', got %v", err)
	}
}

func TestDownloadTelegramVoice_DefaultsToOggExtension(t *testing.T) {
	botApi := &mockBotGetter{
		file: &TelegramFileInfo{FilePath: "voice/file_0"},
	}
	botToken := "test-bot-token"
	voice := TelegramVoiceInfo{
		FileID:       "voice-file-id-123",
		FileUniqueID: "unique-456",
	}

	mockHTTP := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBuffer(make([]byte, 50))),
			}, nil
		}),
	}

	result, err := DownloadTelegramVoice(context.Background(), botApi, botToken, voice, mockHTTP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = os.Remove(result) }()

	matched, _ := regexp.MatchString(`\.ogg$`, result)
	if !matched {
		t.Errorf("expected path to end in .ogg, got %s", result)
	}
}

func TestTranscribeVoice_ReturnsNilAndCleansUpFileOnFailure(t *testing.T) {
	// Create a temporary file to transcribe
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test-voice.ogg")
	if err := os.WriteFile(tempFile, []byte("fake-ogg-content"), 0o600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	failingRunner := func(_ context.Context, _ string) (string, error) {
		return "", errors.New("whisper-cli failed")
	}

	result := TranscribeVoice(context.Background(), tempFile, failingRunner)
	if result != nil {
		t.Fatalf("expected nil result on failure, got %v", *result)
	}

	// Verify temp file was removed
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Errorf("expected file %s to be cleaned up, but it still exists", tempFile)
	}
}

func TestTranscribeVoice_SuccessAndCleansUpFile(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test-voice.ogg")
	if err := os.WriteFile(tempFile, []byte("fake-ogg-content"), 0o600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	successRunner := func(_ context.Context, _ string) (string, error) {
		return "[00:00:00.000 --> 00:00:02.000] Hello from voice message!", nil
	}

	result := TranscribeVoice(context.Background(), tempFile, successRunner)
	if result == nil {
		t.Fatal("expected non-nil result on success")
	}
	if *result != "Hello from voice message!" {
		t.Errorf("expected 'Hello from voice message!', got '%s'", *result)
	}

	// Verify temp file was removed
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Errorf("expected file %s to be cleaned up, but it still exists", tempFile)
	}
}

func TestStripWhisperTimestamps_RemovesTimestampPrefixes(t *testing.T) {
	raw := "[00:00:00.000 --> 00:00:02.000]   Hello world \n[00:00:02.000 --> 00:00:04.000] test"
	expected := "Hello world \ntest"

	result := StripWhisperTimestamps(raw)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestCheckWhisperAvailability(t *testing.T) {
	t.Run("returns MsgWhisperNotInstalled when package dir is missing", func(t *testing.T) {
		emptyDir := t.TempDir()
		res := CheckWhisperAvailability(WhisperInspector{BaseDir: emptyDir})
		if res == nil || *res != MsgWhisperNotInstalled {
			t.Fatalf("expected MsgWhisperNotInstalled, got %v", res)
		}
	})

	t.Run("returns MsgWhisperNotCompiled when binary is missing", func(t *testing.T) {
		tempDir := t.TempDir()
		pkgDir := filepath.Join(tempDir, "node_modules", "nodejs-whisper")
		_ = os.MkdirAll(pkgDir, 0o755)

		res := CheckWhisperAvailability(WhisperInspector{BaseDir: tempDir})
		if res == nil || *res != MsgWhisperNotCompiled {
			t.Fatalf("expected MsgWhisperNotCompiled, got %v", res)
		}
	})

	t.Run("returns MsgWhisperModelMissing when model is missing", func(t *testing.T) {
		tempDir := t.TempDir()
		pkgDir := filepath.Join(tempDir, "node_modules", "nodejs-whisper")
		binDir := filepath.Join(pkgDir, "cpp", "whisper.cpp", "build", "bin")
		_ = os.MkdirAll(binDir, 0o755)
		_ = os.WriteFile(filepath.Join(binDir, "whisper-cli"), []byte("bin"), 0o755)
		_ = os.MkdirAll(filepath.Join(pkgDir, "models"), 0o755)

		res := CheckWhisperAvailability(WhisperInspector{BaseDir: tempDir})
		if res == nil || *res != MsgWhisperModelMissing {
			t.Fatalf("expected MsgWhisperModelMissing, got %v", res)
		}
	})

	t.Run("returns nil when all components exist", func(t *testing.T) {
		tempDir := t.TempDir()
		pkgDir := filepath.Join(tempDir, "node_modules", "nodejs-whisper")
		binDir := filepath.Join(pkgDir, "cpp", "whisper.cpp", "build", "bin")
		modelsDir := filepath.Join(pkgDir, "models")
		_ = os.MkdirAll(binDir, 0o755)
		_ = os.MkdirAll(modelsDir, 0o755)
		_ = os.WriteFile(filepath.Join(binDir, "whisper-cli"), []byte("bin"), 0o755)
		_ = os.WriteFile(filepath.Join(modelsDir, "ggml-base.en.bin"), []byte("model"), 0o644)

		res := CheckWhisperAvailability(WhisperInspector{BaseDir: tempDir})
		if res != nil {
			t.Fatalf("expected nil when all ready, got %v", *res)
		}
	})
}
