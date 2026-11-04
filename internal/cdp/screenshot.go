package cdp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"antigravity-telegram-remote/internal/logger"
)

// CDPCaller defines the minimal interface consumed by ScreenshotService.
type CDPCaller interface {
	Call(method string, params any) (json.RawMessage, error)
}

// ClipRegion represents the capture clipping rectangle (src/services/screenshotService.ts:11-17).
type ClipRegion struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Scale  float64 `json:"scale"`
}

// CaptureOptions mirrors CaptureOptions (src/services/screenshotService.ts:5-20).
type CaptureOptions struct {
	Format                string
	Quality               *int
	Clip                  *ClipRegion
	CaptureBeyondViewport *bool
}

// CaptureResult mirrors CaptureResult (src/services/screenshotService.ts:23-30).
type CaptureResult struct {
	Success bool
	Buffer  []byte
	Error   string
}

// ScreenshotServiceOptions mirrors ScreenshotServiceOptions (src/services/screenshotService.ts:32-35).
type ScreenshotServiceOptions struct {
	CDPService CDPCaller
}

// ScreenshotService mirrors ScreenshotService (src/services/screenshotService.ts:43-128).
type ScreenshotService struct {
	cdpService CDPCaller
}

// NewScreenshotService creates a new ScreenshotService instance.
func NewScreenshotService(opts ScreenshotServiceOptions) *ScreenshotService {
	return &ScreenshotService{
		cdpService: opts.CDPService,
	}
}

// Capture captures a screenshot using CDP Page.captureScreenshot.
func (s *ScreenshotService) Capture(opts ...CaptureOptions) CaptureResult {
	var opt CaptureOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	format := opt.Format
	if format == "" {
		format = "png"
	}

	params := map[string]any{
		"format": format,
	}
	if opt.Quality != nil {
		params["quality"] = *opt.Quality
	}
	if opt.Clip != nil {
		params["clip"] = opt.Clip
	}
	if opt.CaptureBeyondViewport != nil {
		params["captureBeyondViewport"] = *opt.CaptureBeyondViewport
	}

	resRaw, err := s.cdpService.Call("Page.captureScreenshot", params)
	if err != nil {
		logger.Error(fmt.Sprintf("[ScreenshotService] Error during capture: %v", err))
		return CaptureResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	var res struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(resRaw, &res); err != nil {
		return CaptureResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	if res.Data == "" {
		return CaptureResult{
			Success: false,
			Error:   "Screenshot data was empty.",
		}
	}

	buffer, err := base64.StdEncoding.DecodeString(res.Data)
	if err != nil {
		return CaptureResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	return CaptureResult{
		Success: true,
		Buffer:  buffer,
	}
}

// GetBase64 retrieves the base64-encoded screenshot string directly.
func (s *ScreenshotService) GetBase64(opts ...CaptureOptions) *string {
	var opt CaptureOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	format := opt.Format
	if format == "" {
		format = "png"
	}

	params := map[string]any{
		"format": format,
	}
	if opt.Quality != nil {
		params["quality"] = *opt.Quality
	}
	if opt.Clip != nil {
		params["clip"] = opt.Clip
	}
	if opt.CaptureBeyondViewport != nil {
		params["captureBeyondViewport"] = *opt.CaptureBeyondViewport
	}

	resRaw, err := s.cdpService.Call("Page.captureScreenshot", params)
	if err != nil {
		logger.Error(fmt.Sprintf("[ScreenshotService] Error while getting Base64: %v", err))
		return nil
	}

	var res struct {
		Data *string `json:"data"`
	}
	if err := json.Unmarshal(resRaw, &res); err != nil || res.Data == nil {
		return nil
	}

	return res.Data
}
