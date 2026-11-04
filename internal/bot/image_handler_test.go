package bot

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockFileResolver struct {
	paths map[string]string
	err   error
}

func (m *mockFileResolver) GetFile(_ context.Context, fileID string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.paths[fileID], nil
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestIsImageAttachment_TrueWhenContentTypeImage(t *testing.T) {
	if !IsImageAttachment("image/png", "any.bin") {
		t.Error("expected true when contentType is image/png")
	}
	if !IsImageAttachment("IMAGE/JPEG", "") {
		t.Error("expected true when contentType is uppercase IMAGE/JPEG")
	}
}

func TestIsImageAttachment_TrueWhenExtensionIsImage(t *testing.T) {
	if !IsImageAttachment("", "photo.jpeg") {
		t.Error("expected true for photo.jpeg with empty content type")
	}
	if !IsImageAttachment(nilString(), "diagram.png") {
		t.Error("expected true for diagram.png")
	}
	if !IsImageAttachment("", "graphic.webp") {
		t.Error("expected true for graphic.webp")
	}
	if !IsImageAttachment("", "anim.GIF") {
		t.Error("expected true for anim.GIF")
	}
	if !IsImageAttachment("", "icon.bmp") {
		t.Error("expected true for icon.bmp")
	}
}

func nilString() string {
	return ""
}

func TestIsImageAttachment_FalseForNonImage(t *testing.T) {
	if IsImageAttachment("application/pdf", "doc.pdf") {
		t.Error("expected false for application/pdf")
	}
	if IsImageAttachment("text/plain", "notes.txt") {
		t.Error("expected false for text/plain")
	}
	if IsImageAttachment("", "archive.zip") {
		t.Error("expected false for archive.zip")
	}
}

func TestMimeTypeToExtension_ConvertsCommonMimeTypes(t *testing.T) {
	if ext := MimeTypeToExtension("image/png"); ext != "png" {
		t.Errorf("expected 'png', got '%s'", ext)
	}
	if ext := MimeTypeToExtension("image/jpeg"); ext != "jpg" {
		t.Errorf("expected 'jpg', got '%s'", ext)
	}
	if ext := MimeTypeToExtension("image/jpg"); ext != "jpg" {
		t.Errorf("expected 'jpg', got '%s'", ext)
	}
	if ext := MimeTypeToExtension("image/webp"); ext != "webp" {
		t.Errorf("expected 'webp', got '%s'", ext)
	}
	if ext := MimeTypeToExtension("image/gif"); ext != "gif" {
		t.Errorf("expected 'gif', got '%s'", ext)
	}
	if ext := MimeTypeToExtension("image/bmp"); ext != "bmp" {
		t.Errorf("expected 'bmp', got '%s'", ext)
	}
	if ext := MimeTypeToExtension("unknown/format"); ext != "png" {
		t.Errorf("expected default 'png', got '%s'", ext)
	}
}

func TestSanitizeFileName_ReplacesInvalidCharactersWithHyphens(t *testing.T) {
	result := SanitizeFileName("bad name/with*chars?.png")
	expected := "bad-name-with-chars-.png"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}

	result2 := SanitizeFileName("---multiple---hyphens---.jpg")
	expected2 := "multiple-hyphens-.jpg"
	if result2 != expected2 {
		t.Errorf("expected %q, got %q", expected2, result2)
	}

	emptySanitized := SanitizeFileName("???")
	if !strings.HasPrefix(emptySanitized, "image-") || !strings.HasSuffix(emptySanitized, ".png") {
		t.Errorf("expected image-<timestamp>.png fallback for empty, got %q", emptySanitized)
	}
}

func TestBuildPromptWithAttachmentUrls_FormatsPromptAndAttachments(t *testing.T) {
	attachments := []InboundImageAttachment{
		{Name: "photo1.png", URL: "[local file: photo1.png]"},
		{Name: "photo2.jpg", URL: "[local file: photo2.jpg]"},
	}

	// With user prompt
	prompt := "Explain this diagram"
	res := BuildPromptWithAttachmentUrls(prompt, attachments)
	expectedPrefix := "Explain this diagram\n\n[Telegram Attached Images]\n1. photo1.png\nURL: [local file: photo1.png]\n\n2. photo2.jpg\nURL: [local file: photo2.jpg]"
	if !strings.HasPrefix(res, expectedPrefix) {
		t.Errorf("unexpected formatted prompt:\n%s", res)
	}
	if !strings.HasSuffix(res, ImagePromptFooter) {
		t.Errorf("expected footer %q at end of prompt", ImagePromptFooter)
	}

	// With empty user prompt -> default prompt base
	resEmpty := BuildPromptWithAttachmentUrls("", attachments)
	if !strings.HasPrefix(resEmpty, DefaultImagePromptBase) {
		t.Errorf("expected default prompt base for empty prompt, got %s", resEmpty)
	}

	// With empty attachments -> returns base prompt
	resNoAttach := BuildPromptWithAttachmentUrls("Just text", nil)
	if resNoAttach != "Just text" {
		t.Errorf("expected 'Just text', got %q", resNoAttach)
	}
}

func TestToTelegramInputFile_DecodesBase64AndAppendsExtension(t *testing.T) {
	t.Run("decodes raw base64 and adds extension when missing", func(t *testing.T) {
		payload := []byte("fake-png-binary-bytes")
		encoded := base64.StdEncoding.EncodeToString(payload)

		img := ExtractedResponseImage{
			Name:       "diagram_result",
			MimeType:   "image/jpeg",
			Base64Data: encoded,
		}

		res := ToTelegramInputFile(context.Background(), img, 0)
		if res == nil {
			t.Fatal("expected non-nil TelegramInputFileResult")
		}
		if res.Name != "diagram_result.jpg" {
			t.Errorf("expected diagram_result.jpg, got %s", res.Name)
		}
		if !bytes.Equal(res.Buffer, payload) {
			t.Errorf("decoded buffer mismatch")
		}
	})

	t.Run("preserves existing image extension", func(t *testing.T) {
		payload := []byte("image-data")
		encoded := base64.StdEncoding.EncodeToString(payload)

		img := ExtractedResponseImage{
			Name:       "chart.webp",
			MimeType:   "image/webp",
			Base64Data: encoded,
		}

		res := ToTelegramInputFile(context.Background(), img, 0)
		if res == nil {
			t.Fatal("expected non-nil result")
		}
		if res.Name != "chart.webp" {
			t.Errorf("expected chart.webp, got %s", res.Name)
		}
	})

	t.Run("decodes data URL with prefix", func(t *testing.T) {
		payload := []byte("data-url-bytes")
		dataURL := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(payload))

		img := ExtractedResponseImage{
			Base64Data: dataURL,
		}

		res := ToTelegramInputFile(context.Background(), img, 2)
		if res == nil {
			t.Fatal("expected non-nil result")
		}
		if res.Name != "generated-image-3.png" {
			t.Errorf("expected generated-image-3.png, got %s", res.Name)
		}
		if !bytes.Equal(res.Buffer, payload) {
			t.Errorf("decoded buffer mismatch")
		}
	})
}

func TestToTelegramInputFile_RemoteHTTP(t *testing.T) {
	mockHTTP := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == "https://example.com/image.png" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"image/png"},
					},
					Body: io.NopCloser(bytes.NewBufferString("remote-image-bytes")),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString("not found")),
			}, nil
		}),
	}

	img := ExtractedResponseImage{
		Name: "downloaded",
		URL:  "https://example.com/image.png",
	}

	res := ToTelegramInputFile(context.Background(), img, 0, mockHTTP)
	if res == nil {
		t.Fatal("expected non-nil result for remote image")
	}
	if res.Name != "downloaded.png" {
		t.Errorf("expected downloaded.png, got %s", res.Name)
	}
	if string(res.Buffer) != "remote-image-bytes" {
		t.Errorf("expected buffer 'remote-image-bytes', got %s", string(res.Buffer))
	}
}

func TestDownloadTelegramImages_DownloadsUpToFourPhotosAndCleansUp(t *testing.T) {
	photos := []TelegramPhotoInfo{
		{FileID: "id-1"},
		{FileID: "id-2"},
		{FileID: "id-3"},
		{FileID: "id-4"},
		{FileID: "id-5"}, // Should be ignored (max 4)
	}

	resolver := &mockFileResolver{
		paths: map[string]string{
			"id-1": "photos/photo1.jpg",
			"id-2": "photos/photo2.png",
			"id-3": "photos/photo3.webp",
			"id-4": "photos/photo4.gif",
		},
	}

	mockHTTP := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("photo-content")),
			}, nil
		}),
	}

	attachments, err := DownloadTelegramImages(context.Background(), resolver, "bot-token", photos, "msg-123", mockHTTP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(attachments) != 4 {
		t.Fatalf("expected 4 attachments, got %d", len(attachments))
	}

	for _, att := range attachments {
		if _, err := os.Stat(att.LocalPath); err != nil {
			t.Errorf("file %s does not exist on disk: %v", att.LocalPath, err)
		}
	}

	// Verify cleanup
	CleanupInboundImageAttachments(attachments)
	for _, att := range attachments {
		if _, err := os.Stat(att.LocalPath); !os.IsNotExist(err) {
			t.Errorf("file %s was not cleaned up", att.LocalPath)
		}
	}
}

func TestImageToDataURL_And_FileToDataURL(t *testing.T) {
	data := []byte("hello-image")
	dataURL := ImageToDataURL("image/png", data)
	expectedPrefix := "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, expectedPrefix) {
		t.Errorf("expected data URL to start with %s, got %s", expectedPrefix, dataURL)
	}

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "sample.jpg")
	if err := os.WriteFile(tempFile, data, 0o600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	fileDataURL, err := FileToDataURL(tempFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(fileDataURL, "data:image/jpeg;base64,") {
		t.Errorf("expected jpeg data URL for .jpg file, got %s", fileDataURL)
	}
}
