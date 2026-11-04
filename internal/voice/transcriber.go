package voice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	// TempVoiceSubdir is the subdirectory inside os.TempDir() for downloaded voice messages.
	TempVoiceSubdir = "antigravity-telegram-remote-voice"

	// DefaultVoiceExt is the fallback extension when Telegram file_path lacks an extension.
	DefaultVoiceExt = ".ogg"

	// DefaultWhisperModelName is the expected Whisper model filename prefix.
	DefaultWhisperModelName = "base.en"

	// Setup messages matching the original TypeScript implementation.
	MsgWhisperNotInstalled = "🔇 Voice transcription is not set up.\n\n" +
		"To enable it, run:\n" +
		"  npm install nodejs-whisper ffmpeg-static\n" +
		"  npx nodejs-whisper download"

	MsgWhisperNotCompiled = "🔇 Whisper is installed but not compiled.\n\n" +
		"Run this to build it:\n" +
		"  cd node_modules/nodejs-whisper/cpp/whisper.cpp\n" +
		"  cmake -B build && cmake --build build --config Release\n\n" +
		"Requires cmake (brew install cmake / apt install cmake)."

	MsgWhisperModelMissing = "🔇 Whisper model not downloaded.\n\n" +
		"Run this once to fetch the base.en model (~140 MB):\n" +
		"  npx nodejs-whisper download"
)

// WhisperTimestampRegex matches Whisper timestamp prefixes like "[00:00:00.000 --> 00:00:02.000] ".
var WhisperTimestampRegex = regexp.MustCompile(`\[[\d:.]+\s*-->\s*[\d:.]+\]\s*`)

// TelegramVoiceInfo represents voice message metadata from a Telegram message.
type TelegramVoiceInfo struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Duration     int    `json:"duration"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

// TelegramFileInfo represents the file object returned by getFile.
type TelegramFileInfo struct {
	FilePath string `json:"file_path"`
}

// TelegramFileGetter abstracts the Telegram bot getFile API call.
type TelegramFileGetter interface {
	GetFile(ctx context.Context, fileID string) (*TelegramFileInfo, error)
}

// WhisperInspector provides configuration for checking Whisper availability.
type WhisperInspector struct {
	BaseDir           string
	WhisperBinaryPath string
	WhisperModelDir   string
}

// WhisperRunner abstracts audio transcription execution for testing and pluggability.
type WhisperRunner func(ctx context.Context, voicePath string) (string, error)

// TempVoiceDir returns the path to the temporary directory for voice audio files.
func TempVoiceDir() string {
	return filepath.Join(os.TempDir(), TempVoiceSubdir)
}

// CheckWhisperAvailability verifies the Whisper environment sequentially.
// Returns nil if ready, or a user-facing instruction message if not.
func CheckWhisperAvailability(inspector ...WhisperInspector) *string {
	var insp WhisperInspector
	if len(inspector) > 0 {
		insp = inspector[0]
	}
	baseDir := insp.BaseDir
	if baseDir == "" {
		baseDir = "."
	}

	// 1. Check package directory (or custom binary)
	pkgDir := filepath.Join(baseDir, "node_modules", "nodejs-whisper")
	if insp.WhisperBinaryPath == "" {
		stat, err := os.Stat(pkgDir)
		if err != nil || !stat.IsDir() {
			msg := MsgWhisperNotInstalled
			return &msg
		}
	}

	// 2. Check whisper-cli binary
	binPath := insp.WhisperBinaryPath
	if binPath == "" {
		binPath = filepath.Join(pkgDir, "cpp", "whisper.cpp", "build", "bin", "whisper-cli")
		if runtime.GOOS == "windows" {
			binPath += ".exe"
		}
	}
	stat, err := os.Stat(binPath)
	if err != nil || !stat.Mode().IsRegular() {
		msg := MsgWhisperNotCompiled
		return &msg
	}

	// 3. Check model file
	modelsDir := insp.WhisperModelDir
	if modelsDir == "" {
		modelsDir = filepath.Join(pkgDir, "models")
	}
	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		msg := MsgWhisperModelMissing
		return &msg
	}
	found := false
	for _, f := range entries {
		if strings.Contains(f.Name(), DefaultWhisperModelName) {
			found = true
			break
		}
	}
	if !found {
		msg := MsgWhisperModelMissing
		return &msg
	}

	return nil
}

// DownloadTelegramVoice downloads a voice message from the Telegram Bot API to a local temp file.
func DownloadTelegramVoice(ctx context.Context, botApi TelegramFileGetter, botToken string, voice TelegramVoiceInfo, httpClient ...*http.Client) (string, error) {
	if err := os.MkdirAll(TempVoiceDir(), 0o755); err != nil {
		return "", err
	}

	file, err := botApi.GetFile(ctx, voice.FileID)
	if err != nil {
		return "", err
	}
	if file == nil || file.FilePath == "" {
		return "", errors.New("Telegram returned no file_path for voice message")
	}

	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, file.FilePath)

	client := http.DefaultClient
	if len(httpClient) > 0 && httpClient[0] != nil {
		client = httpClient[0]
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Voice download failed (status=%d)", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errors.New("Voice download returned empty file")
	}

	ext := filepath.Ext(file.FilePath)
	if ext == "" {
		ext = DefaultVoiceExt
	}

	localPath := filepath.Join(TempVoiceDir(), fmt.Sprintf("%d-%s%s", time.Now().UnixMilli(), voice.FileUniqueID, ext))
	if err := os.WriteFile(localPath, data, 0o600); err != nil {
		return "", err
	}

	slog.Info(fmt.Sprintf("[VoiceHandler] Downloaded voice message to %s (%d bytes)", localPath, len(data)))
	return localPath, nil
}

func defaultWhisperRunner(ctx context.Context, voicePath string) (string, error) {
	ffmpegPath := os.Getenv("FFMPEG_PATH")
	if ffmpegPath == "" {
		var err error
		ffmpegPath, err = exec.LookPath("ffmpeg")
		if err != nil {
			return "", fmt.Errorf("ffmpeg not found: %w", err)
		}
	}

	wavPath := voicePath + ".wav"
	defer func() { _ = os.Remove(wavPath) }()

	convCmd := exec.CommandContext(ctx, ffmpegPath, "-y", "-i", voicePath, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wavPath)
	if out, err := convCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("audio conversion failed: %s: %w", string(out), err)
	}

	whisperCli := os.Getenv("WHISPER_CLI_PATH")
	if whisperCli == "" {
		var err error
		whisperCli, err = exec.LookPath("whisper-cli")
		if err != nil {
			return "", fmt.Errorf("whisper-cli not found: %w", err)
		}
	}

	modelPath := os.Getenv("WHISPER_MODEL_PATH")
	args := []string{"-f", wavPath, "-nt"}
	if modelPath != "" {
		args = append([]string{"-m", modelPath}, args...)
	}

	whisperCmd := exec.CommandContext(ctx, whisperCli, args...)
	out, err := whisperCmd.Output()
	if err != nil {
		return "", fmt.Errorf("whisper execution failed: %w", err)
	}

	return string(out), nil
}

// TranscribeVoice transcribes a local voice file, returning the text transcript or nil on failure.
// Always cleans up voicePath upon return.
func TranscribeVoice(ctx context.Context, voicePath string, runner ...WhisperRunner) *string {
	defer func() {
		_ = os.Remove(voicePath)
	}()

	var raw string
	var err error

	if len(runner) > 0 && runner[0] != nil {
		raw, err = runner[0](ctx, voicePath)
	} else {
		raw, err = defaultWhisperRunner(ctx, voicePath)
	}

	if err != nil {
		slog.Error("[VoiceHandler] Transcription failed:", "error", err)
		return nil
	}

	transcript := StripWhisperTimestamps(raw)
	if transcript == "" {
		slog.Warn("[VoiceHandler] Whisper returned empty transcript")
		return nil
	}

	preview := transcript
	if len(preview) > 100 {
		preview = preview[:100] + "..."
	}
	slog.Info(fmt.Sprintf("[VoiceHandler] Transcribed: %q", preview))
	return &transcript
}

// StripWhisperTimestamps removes prefix timestamps such as "[00:00:00.000 --> 00:00:02.000] ".
func StripWhisperTimestamps(raw string) string {
	trimmed := strings.TrimSpace(raw)
	stripped := WhisperTimestampRegex.ReplaceAllString(trimmed, "")
	return strings.TrimSpace(stripped)
}
