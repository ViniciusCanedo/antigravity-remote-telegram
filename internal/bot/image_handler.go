package bot

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// MaxInboundImageAttachments limits maximum photos processed per message.
	MaxInboundImageAttachments = 4

	// TempImageSubdir is the subdirectory inside os.TempDir() for downloaded images.
	TempImageSubdir = "antigravity-telegram-remote-images"

	// DefaultImagePromptBase is used when the user sends an image without caption or text.
	DefaultImagePromptBase = "Please review the attached images and respond accordingly."

	// ImagePromptFooter is appended to prompts with attached image references.
	ImagePromptFooter = "Please refer to the attached images above in your response."
)

// Supported image filename pattern.
var ImageExtPattern = regexp.MustCompile(`(?i)\.(png|jpe?g|webp|gif|bmp)$`)

// InvalidFileNameCharsRegex matches characters outside [a-zA-Z0-9._-].
var InvalidFileNameCharsRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// MultipleHyphensRegex matches multiple consecutive hyphens.
var MultipleHyphensRegex = regexp.MustCompile(`-+`)

// HTTPURLPrefixRegex matches http:// or https:// URL prefixes.
var HTTPURLPrefixRegex = regexp.MustCompile(`(?i)^https?://`)

// InboundImageAttachment represents a downloaded Telegram photo ready for CDP upload.
type InboundImageAttachment struct {
	LocalPath string `json:"localPath"`
	URL       string `json:"url"`
	Name      string `json:"name"`
	MimeType  string `json:"mimeType"`
}

// TelegramPhotoInfo represents a photo item in a Telegram message.
type TelegramPhotoInfo struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int64  `json:"file_size,omitempty"`
}

// ExtractedResponseImage represents an image extracted from Antigravity CDP response.
type ExtractedResponseImage struct {
	Name       string `json:"name,omitempty"`
	MimeType   string `json:"mimeType,omitempty"`
	Base64Data string `json:"base64Data,omitempty"`
	URL        string `json:"url,omitempty"`
}

// TelegramInputFileResult contains the binary payload and target filename for sending to Telegram.
type TelegramInputFileResult struct {
	Buffer []byte
	Name   string
}

// TelegramFileResolver abstracts bot API getFile calls for images.
type TelegramFileResolver interface {
	GetFile(ctx context.Context, fileID string) (filePath string, err error)
}

// TempImageDir returns the directory path for temporary inbound images.
func TempImageDir() string {
	return filepath.Join(os.TempDir(), TempImageSubdir)
}

// IsImageAttachment returns true if contentType is image/* or the filename ends with an image extension.
func IsImageAttachment(contentType string, fileName string) bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "image/") {
		return true
	}
	return ImageExtPattern.MatchString(fileName)
}

// MimeTypeToExtension maps MIME types to standard file extensions (without leading dot).
func MimeTypeToExtension(mimeType string) string {
	normalized := strings.ToLower(mimeType)
	if strings.Contains(normalized, "jpeg") || strings.Contains(normalized, "jpg") {
		return "jpg"
	}
	if strings.Contains(normalized, "webp") {
		return "webp"
	}
	if strings.Contains(normalized, "gif") {
		return "gif"
	}
	if strings.Contains(normalized, "bmp") {
		return "bmp"
	}
	return "png"
}

// SanitizeFileName replaces unsafe characters with hyphens and cleans edge hyphens.
func SanitizeFileName(fileName string) string {
	sanitized := InvalidFileNameCharsRegex.ReplaceAllString(fileName, "-")
	sanitized = MultipleHyphensRegex.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		return fmt.Sprintf("image-%d.png", time.Now().UnixMilli())
	}
	return sanitized
}

// BuildPromptWithAttachmentUrls formats a user prompt with attached image references.
func BuildPromptWithAttachmentUrls(prompt string, attachments []InboundImageAttachment) string {
	base := strings.TrimSpace(prompt)
	if base == "" {
		base = DefaultImagePromptBase
	}
	if len(attachments) == 0 {
		return base
	}

	lines := make([]string, len(attachments))
	for i, img := range attachments {
		lines[i] = fmt.Sprintf("%d. %s\nURL: %s", i+1, img.Name, img.URL)
	}

	return fmt.Sprintf("%s\n\n[Telegram Attached Images]\n%s\n\n%s", base, strings.Join(lines, "\n\n"), ImagePromptFooter)
}

// ImageToDataURL converts raw image bytes into a data URL (data:image/png;base64,...).
func ImageToDataURL(mimeType string, data []byte) string {
	if mimeType == "" {
		mimeType = "image/png"
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
}

// FileToDataURL reads a local image file and converts it into a data URL.
func FileToDataURL(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	var mimeType string
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".webp":
		mimeType = "image/webp"
	case ".gif":
		mimeType = "image/gif"
	case ".bmp":
		mimeType = "image/bmp"
	default:
		mimeType = "image/png"
	}
	return ImageToDataURL(mimeType, data), nil
}

// DownloadTelegramImages downloads up to 4 images from a Telegram message to local temp storage.
func DownloadTelegramImages(ctx context.Context, botApi TelegramFileResolver, botToken string, photos []TelegramPhotoInfo, messageID string, httpClient ...*http.Client) ([]InboundImageAttachment, error) {
	limit := len(photos)
	if limit > MaxInboundImageAttachments {
		limit = MaxInboundImageAttachments
	}
	if limit == 0 {
		return []InboundImageAttachment{}, nil
	}
	selected := photos[:limit]

	if err := os.MkdirAll(TempImageDir(), 0o755); err != nil {
		return nil, err
	}

	client := http.DefaultClient
	if len(httpClient) > 0 && httpClient[0] != nil {
		client = httpClient[0]
	}

	downloaded := make([]InboundImageAttachment, 0, limit)
	index := 0

	for _, photo := range selected {
		filePath, err := botApi.GetFile(ctx, photo.FileID)
		if err != nil || filePath == "" {
			continue
		}

		url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, filePath)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			slog.Warn(fmt.Sprintf("[ImageBridge] Telegram image download failed (status=%d)", resp.StatusCode))
			continue
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || len(data) == 0 {
			continue
		}

		ext := filepath.Ext(filePath)
		if ext == "" {
			ext = ".png"
		}

		name := SanitizeFileName(fmt.Sprintf("telegram-image-%d%s", index+1, ext))
		localPath := filepath.Join(TempImageDir(), fmt.Sprintf("%d-%s-%d-%s", time.Now().UnixMilli(), messageID, index, name))

		if err := os.WriteFile(localPath, data, 0o600); err != nil {
			continue
		}

		mimeExt := strings.ToLower(strings.TrimPrefix(ext, "."))
		if mimeExt == "jpg" {
			mimeExt = "jpeg"
		}

		downloaded = append(downloaded, InboundImageAttachment{
			LocalPath: localPath,
			URL:       fmt.Sprintf("[local file: %s]", name),
			Name:      name,
			MimeType:  "image/" + mimeExt,
		})
		index++
	}

	return downloaded, nil
}

// CleanupInboundImageAttachments removes temporary inbound image files from disk.
func CleanupInboundImageAttachments(attachments []InboundImageAttachment) {
	for _, img := range attachments {
		_ = os.Remove(img.LocalPath)
	}
}

// ToTelegramInputFile converts an ExtractedResponseImage into a buffer and filename for Telegram sending.
func ToTelegramInputFile(ctx context.Context, image ExtractedResponseImage, index int, httpClient ...*http.Client) *TelegramInputFileResult {
	var buffer []byte
	mimeType := image.MimeType
	if mimeType == "" {
		mimeType = "image/png"
	}

	if image.Base64Data != "" {
		data := image.Base64Data
		// Support data URLs: data:image/...;base64,...
		if idx := strings.Index(data, ","); idx != -1 && strings.HasPrefix(data, "data:") {
			header := data[:idx]
			data = data[idx+1:]
			if image.MimeType == "" {
				// Extract mime from header: data:<mime>;base64
				trimmed := strings.TrimPrefix(header, "data:")
				if semi := strings.Index(trimmed, ";"); semi != -1 {
					mimeType = trimmed[:semi]
				}
			}
		}

		decoded, err := base64.StdEncoding.DecodeString(data)
		if err == nil {
			buffer = decoded
		}
	} else if image.URL != "" && HTTPURLPrefixRegex.MatchString(image.URL) {
		client := http.DefaultClient
		if len(httpClient) > 0 && httpClient[0] != nil {
			client = httpClient[0]
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, image.URL, nil)
		if err == nil {
			resp, err := client.Do(req)
			if err == nil {
				if resp.StatusCode == http.StatusOK {
					b, err := io.ReadAll(resp.Body)
					if err == nil {
						buffer = b
						if ct := resp.Header.Get("Content-Type"); ct != "" {
							mimeType = ct
						}
					}
				}
				resp.Body.Close()
			}
		}
	}

	if len(buffer) == 0 {
		return nil
	}

	fallbackExt := MimeTypeToExtension(mimeType)
	rawName := image.Name
	if rawName == "" {
		rawName = fmt.Sprintf("generated-image-%d.%s", index+1, fallbackExt)
	}

	baseName := SanitizeFileName(rawName)
	finalName := baseName
	if !ImageExtPattern.MatchString(baseName) {
		finalName = fmt.Sprintf("%s.%s", baseName, fallbackExt)
	}

	return &TelegramInputFileResult{
		Buffer: buffer,
		Name:   finalName,
	}
}
