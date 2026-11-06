package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/logger"
	"antigravity-telegram-remote/internal/monitor"
)

// SelectorCheckEntry defines a named selector checked by DoctorSelector.
type SelectorCheckEntry struct {
	Name       string
	Expression string
}

// SelectorEvalResult maps the JSON envelope returned by the DoctorSelector IIFE wrapper:
// { ok: true, data: result } or { ok: false, error: e.message }.
type SelectorEvalResult struct {
	Ok    bool   `json:"ok"`
	Data  any    `json:"data"`
	Error string `json:"error,omitempty"`
}

// DoctorSelectorReport summarizes the results of the selector health check.
type DoctorSelectorReport struct {
	HealthCount int
	TotalCount  int
}

// CdpCaller abstracts the required methods of cdp.Client for DoctorSelector.
type CdpCaller interface {
	Connect() error
	Disconnect() error
	IsConnected() bool
	Call(method string, params any) (json.RawMessage, error)
}

// DoctorSelectorChecks contains the 7 core selectors checked in deterministic order.
var DoctorSelectorChecks = []SelectorCheckEntry{
	{Name: "Chat Message (Text)", Expression: monitor.ResponseSelectors.ResponseText},
	{Name: "Stop Button (State)", Expression: monitor.ResponseSelectors.StopButton},
	{Name: "Planning Active (Dialog)", Expression: monitor.ResponseSelectors.PlanningActive},
	{Name: "Process Logs (Activity)", Expression: monitor.ResponseSelectors.ProcessLogs},
	{Name: "Combined Poll (Logic)", Expression: monitor.ResponseSelectors.CombinedPoll},
	{Name: "Quota Error (Detection)", Expression: monitor.ResponseSelectors.QuotaError},
	{Name: "DOM Observer (Event-driven)", Expression: monitor.ResponseSelectors.DomObserver},
}

// RunDoctorSelectors connects to Antigravity via CDP and audits the 7 core selectors,
// logging the health report to standard outputs.
func RunDoctorSelectors(ctx context.Context) (*DoctorSelectorReport, error) {
	client := cdp.NewClient(cdp.ClientOptions{
		PortsToScan: cdp.DefaultCDPPorts,
	})
	return RunDoctorSelectorsWithDeps(ctx, client, logger.Default, os.Stdout)
}

// RunDoctorSelectorsWithDeps connects to Antigravity using the provided CdpCaller,
// logger, and output writer, allowing full testability without live browser instances.
func RunDoctorSelectorsWithDeps(ctx context.Context, client CdpCaller, log logger.Logger, out io.Writer) (*DoctorSelectorReport, error) {
	if client == nil {
		return nil, errors.New("cdp client is required")
	}
	if log == nil {
		log = logger.Default
	}
	if out == nil {
		out = os.Stdout
	}

	defer func() {
		_ = client.Disconnect()
	}()

	log.Info("🩺 Start Doctor Selector - Auditing Antigravity UI...")

	if err := client.Connect(); err != nil {
		log.Error(fmt.Sprintf("❌ Could not connect to any Antigravity tab: %s", err.Error()))
		return nil, err
	}

	if !client.IsConnected() {
		log.Error("❌ Connection failed unexpectedly.")
		return nil, errors.New("connection failed unexpectedly")
	}

	healthCount := 0
	totalCount := 0

	for _, check := range DoctorSelectorChecks {
		totalCount++
		log.Info(fmt.Sprintf("Checking: [%s]...", check.Name))

		wrapper := fmt.Sprintf(`(function() { 
    try { 
        const result = %s; 
        return { ok: true, data: result }; 
    } catch(e) { 
        return { ok: false, error: e.message }; 
    } 
})()`, check.Expression)

		rawRes, err := client.Call("Runtime.evaluate", map[string]any{
			"expression":    wrapper,
			"returnByValue": true,
		})
		if err != nil {
			fmt.Fprintf(out, "  ❌ CDP Call Error: %s\n", err.Error())
			continue
		}

		var cdpRes struct {
			Result struct {
				Value *SelectorEvalResult `json:"value"`
			} `json:"result"`
			ExceptionDetails *struct {
				Text      string `json:"text"`
				Exception *struct {
					Description string `json:"description"`
				} `json:"exception"`
			} `json:"exceptionDetails"`
		}

		if err := json.Unmarshal(rawRes, &cdpRes); err != nil {
			fmt.Fprintf(out, "  ❌ CDP Call Error: %s\n", err.Error())
			continue
		}

		val := cdpRes.Result.Value
		if val != nil && val.Ok {
			data := val.Data
			isHealthy := data != nil
			if isHealthy {
				fmt.Fprintln(out, "  ✅ Healthy: Found and extracted data.")
				switch data.(type) {
				case map[string]any, []any:
					dataBytes, _ := json.Marshal(data)
					fmt.Fprintf(out, "    Data: %s\n", string(dataBytes))
				}
				healthCount++
			} else {
				fmt.Fprintln(out, "  ⚠️  Warning: Found but returned null/undefined.")
			}
		} else {
			errMsg := "Unknown evaluation error"
			if val != nil && val.Error != "" {
				errMsg = val.Error
			} else if cdpRes.ExceptionDetails != nil {
				if cdpRes.ExceptionDetails.Exception != nil && cdpRes.ExceptionDetails.Exception.Description != "" {
					errMsg = cdpRes.ExceptionDetails.Exception.Description
				} else if cdpRes.ExceptionDetails.Text != "" {
					errMsg = cdpRes.ExceptionDetails.Text
				}
			}
			fmt.Fprintf(out, "  ❌ Broken: %s\n", errMsg)
		}
	}

	fmt.Fprintln(out, "\n--- Final Selector Health Report ---")
	scoreColor := "⚠️"
	if healthCount == totalCount {
		scoreColor = "✨"
	}
	log.Info(fmt.Sprintf("%s Score: %d/%d Healthy", scoreColor, healthCount, totalCount))

	if healthCount == totalCount {
		log.Info("✅ All core selectors are healthy. The bot is in top shape!")
	} else if healthCount > 0 {
		log.Warn("🚨 Some selectors failed. Compatibility might be compromised.")
	} else {
		log.Error("💀 All selectors failed! The UI has likely changed significantly.")
	}

	return &DoctorSelectorReport{
		HealthCount: healthCount,
		TotalCount:  totalCount,
	}, nil
}
