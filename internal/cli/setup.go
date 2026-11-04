package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"antigravity-telegram-remote/internal/config"
	"antigravity-telegram-remote/internal/platform"
	"golang.org/x/term"
)

const TotalSetupSteps = 3

var numericRegex = regexp.MustCompile(`^\d+$`)

// BotInfo represents the data returned by GET https://api.telegram.org/bot<token>/getMe.
type BotInfo struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

// SetupResult contains the 3 values collected by the interactive wizard.
type SetupResult struct {
	TelegramBotToken string
	AllowedUserIDs   []string
	WorkspaceBaseDir string
}

// IsNonEmpty verifies if trimmed string is not empty.
func IsNonEmpty(value string) bool {
	return len(strings.TrimSpace(value)) > 0
}

// IsNumericString verifies if trimmed string consists purely of digits.
func IsNumericString(value string) bool {
	return numericRegex.MatchString(strings.TrimSpace(value))
}

// ParseAllowedUserIDs splits comma-separated string, trims whitespace and removes empty entries.
func ParseAllowedUserIDs(raw string) []string {
	parts := strings.Split(raw, ",")
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

// ValidateAllowedUserIDs validates that at least one ID exists and all IDs are numeric.
func ValidateAllowedUserIDs(raw string) string {
	ids := ParseAllowedUserIDs(raw)
	if len(ids) == 0 {
		return "Vui lòng nhập ít nhất một ID người dùng."
	}
	for _, id := range ids {
		if !IsNumericString(id) {
			return fmt.Sprintf("ID người dùng không hợp lệ: \"%s\" — phải là một chuỗi số.", id)
		}
	}
	return ""
}

// ExpandTilde expands "~" or "~/..." to the user's home directory.
func ExpandTilde(raw string) string {
	if raw == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
		return os.Getenv("HOME")
	}
	if strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			home = os.Getenv("HOME")
		}
		return filepath.Join(home, raw[2:])
	}
	return raw
}

// VerifyTelegramToken queries https://api.telegram.org/bot<token>/getMe with 10s timeout.
func VerifyTelegramToken(ctx context.Context, token string) *BotInfo {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var payload struct {
		Ok     bool     `json:"ok"`
		Result *BotInfo `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}

	if payload.Ok && payload.Result != nil {
		return payload.Result
	}
	return nil
}

// DirectoryCompleter returns candidate subdirectories for auto-completing paths.
func DirectoryCompleter(line string) (candidates []string, origLine string) {
	raw := strings.TrimLeft(line, " \t")
	expanded := ExpandTilde(raw)
	if expanded == "" {
		expanded = "."
	}
	resolved, err := filepath.Abs(expanded)
	if err != nil {
		return nil, line
	}

	var dir string
	var partial string

	fi, statErr := os.Stat(resolved)
	if statErr == nil && fi.IsDir() {
		dir = resolved
		partial = ""
	} else {
		dir = filepath.Dir(resolved)
		partial = filepath.Base(resolved)
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return nil, line
	}

	home, _ := os.UserHomeDir()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if partial != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(partial)) {
			continue
		}

		full := filepath.Join(dir, name) + "/"
		if strings.HasPrefix(raw, "~/") && home != "" && strings.HasPrefix(full, home) {
			rel, relErr := filepath.Rel(home, full)
			if relErr == nil {
				candidates = append(candidates, "~/"+rel)
				continue
			}
		}
		candidates = append(candidates, full)
	}

	return candidates, line
}

// askSecret reads a password from stdin with terminal raw mode and masking.
func askSecret(prompt string, in io.Reader, reader *bufio.Reader, out io.Writer) (string, error) {
	file, isFile := in.(*os.File)
	if isFile && term.IsTerminal(int(file.Fd())) {
		fd := int(file.Fd())
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			return ask(prompt, reader, out)
		}
		defer func() { _ = term.Restore(fd, oldState) }()

		fmt.Fprint(out, prompt)

		var input []rune
		buf := make([]byte, 1)

		for {
			n, rErr := file.Read(buf)
			if n > 0 && rErr == nil {
				b := buf[0]
				if b == '\r' || b == '\n' {
					_ = term.Restore(fd, oldState)
					fmt.Fprint(out, "\n")
					return string(input), nil
				}
				if b == 127 || b == 8 { // Backspace
					if len(input) > 0 {
						input = input[:len(input)-1]
						fmt.Fprint(out, "\b \b")
					}
					continue
				}
				if b == 3 { // Ctrl+C
					_ = term.Restore(fd, oldState)
					fmt.Fprint(out, "\n")
					os.Exit(0)
				}
				if b >= 32 {
					input = append(input, rune(b))
					fmt.Fprint(out, "*")
				}
			} else {
				break
			}
		}
		return string(input), nil
	}

	return ask(prompt, reader, out)
}

func ask(prompt string, reader *bufio.Reader, out io.Writer) (string, error) {
	fmt.Fprint(out, prompt)
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func stepHeader(out io.Writer, step, total int, title string) {
	fmt.Fprintf(out, "  %s[Bước %d/%d]%s %s%s%s\n", ColorCyan, step, total, ColorReset, ColorBold, title, ColorReset)
}

func hint(out io.Writer, text string) {
	fmt.Fprintf(out, "  %s%s%s\n", ColorDim, text, ColorReset)
}

func hintBlank(out io.Writer) {
	fmt.Fprintln(out, "")
}

func errMsg(out io.Writer, text string) {
	fmt.Fprintf(out, "  %s%s%s\n\n", ColorRed, text, ColorReset)
}

// SetupActionWithIO executes the interactive setup wizard with configurable I/O and persistence.
func SetupActionWithIO(ctx context.Context, in io.Reader, out io.Writer, saveFn func(config.PersistedConfig) error) error {
	reader := bufio.NewReader(in)

	fmt.Fprint(out, SetupLogo)
	fmt.Fprintf(out, "  %sHướng dẫn cài đặt — %d bước%s\n\n", ColorBold, TotalSetupSteps, ColorReset)

	// Step 1: Telegram Bot Token
	stepHeader(out, 1, TotalSetupSteps, "Telegram Bot Token")
	hint(out, "1. Mở Telegram và tìm kiếm @BotFather")
	hint(out, "2. Gửi lệnh /newbot và làm theo hướng dẫn để tạo bot mới")
	hint(out, "3. Sao chép mã bot token mà BotFather cung cấp cho bạn")
	hintBlank(out)

	var token string
	for {
		rawToken, err := askSecret(fmt.Sprintf("  %s>%s ", ColorYellow, ColorReset), in, reader, out)
		if err != nil {
			return err
		}
		if !IsNonEmpty(rawToken) {
			errMsg(out, "Token không được để trống.")
			continue
		}
		trimmed := strings.TrimSpace(rawToken)
		fmt.Fprintf(out, "  %sĐang xác thực token...%s", ColorDim, ColorReset)

		botInfo := VerifyTelegramToken(ctx, trimmed)
		if botInfo != nil {
			fmt.Fprintf(out, "\r  %sĐã xác thực!%s Bot: %s@%s%s (%s)\n", ColorGreen, ColorReset, ColorBold, botInfo.Username, ColorReset, botInfo.FirstName)
		} else {
			fmt.Fprintf(out, "\r  %sKhông thể xác thực trực tuyến%s — sử dụng token như hiện tại.\n", ColorYellow, ColorReset)
		}
		token = trimmed
		break
	}
	fmt.Fprintln(out, "")

	// Step 2: Allowed Telegram User IDs
	stepHeader(out, 2, TotalSetupSteps, "ID người dùng Telegram được phép")
	hint(out, "Chỉ những người dùng này mới có thể gửi lệnh cho bot.")
	hint(out, "1. Mở Telegram và tìm kiếm @raw_data_bot")
	hint(out, "2. Gửi bất kỳ tin nhắn nào — nó sẽ trả lời bằng ID người dùng (số) của bạn")
	hint(out, "Nhiều ID: phân tách bằng dấu phẩy (ví dụ: 123456,789012)")
	hintBlank(out)

	var allowedUserIDs []string
	for {
		rawIDs, err := ask(fmt.Sprintf("  %s>%s ", ColorYellow, ColorReset), reader, out)
		if err != nil {
			return err
		}
		valErr := ValidateAllowedUserIDs(rawIDs)
		if valErr == "" {
			allowedUserIDs = ParseAllowedUserIDs(rawIDs)
			break
		}
		errMsg(out, valErr)
	}
	fmt.Fprintln(out, "")

	// Step 3: Workspace Directory
	stepHeader(out, 3, TotalSetupSteps, "Thư mục gốc của Workspace")
	hint(out, "Thư mục cha chứa các dự án code của bạn.")
	hint(out, "Mỗi thư mục con sẽ trở thành một dự án có thể chọn trên Telegram qua lệnh /project.")
	hint(out, "Bạn có thể thay đổi mục này sau tại ~/.antigravity-telegram-remote/config.json hoặc chạy lại lệnh setup.")
	hint(out, "Nhấn Tab để tự động hoàn thành đường dẫn thư mục.")
	hintBlank(out)

	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, "Code")

	var workspaceBaseDir string
	for {
		rawDir, err := ask(fmt.Sprintf("  %s>%s [%s%s%s] ", ColorYellow, ColorReset, ColorDim, defaultDir, ColorReset), reader, out)
		if err != nil {
			return err
		}
		selected := strings.TrimSpace(rawDir)
		if selected == "" {
			selected = defaultDir
		}
		expanded := ExpandTilde(selected)
		resolved, absErr := filepath.Abs(expanded)
		if absErr != nil {
			errMsg(out, "Vui lòng nhập một thư mục hợp lệ.")
			continue
		}

		fi, sErr := os.Stat(resolved)
		if sErr == nil && fi.IsDir() {
			workspaceBaseDir = resolved
			break
		}

		confirmPrompt := fmt.Sprintf("  %s\"%s\" không tồn tại. Tạo thư mục này? (y/n):%s ", ColorYellow, resolved, ColorReset)
		ans, cErr := ask(confirmPrompt, reader, out)
		if cErr == nil && strings.ToLower(strings.TrimSpace(ans)) == "y" {
			if mErr := os.MkdirAll(resolved, 0755); mErr == nil {
				workspaceBaseDir = resolved
				break
			}
		}
		errMsg(out, "Vui lòng nhập một thư mục hợp lệ.")
	}
	fmt.Fprintln(out, "")

	// Save configuration
	if saveFn == nil {
		saveFn = config.Save
	}

	err := saveFn(config.PersistedConfig{
		TelegramBotToken: &token,
		AllowedUserIds:   allowedUserIDs,
		WorkspaceBaseDir: &workspaceBaseDir,
	})
	if err != nil {
		return err
	}

	configPath := config.GetConfigFilePath()

	portsStr := make([]string, len(platform.CDPPorts))
	for i, p := range platform.CDPPorts {
		portsStr[i] = fmt.Sprintf("%d", p)
	}

	fmt.Fprintf(out, "  %sCài đặt hoàn tất!%s\n\n", ColorGreen, ColorReset)
	fmt.Fprintf(out, "  %sĐã lưu tại%s %s\n\n", ColorDim, ColorReset, configPath)
	fmt.Fprintf(out, "  %sCác bước tiếp theo:%s\n", ColorCyan, ColorReset)
	fmt.Fprintf(out, "  %s1.%s Mở Antigravity với CDP được bật:\n", ColorBold, ColorReset)
	fmt.Fprintf(out, "     %santigravity-telegram-remote open%s\n", ColorGreen, ColorReset)
	fmt.Fprintf(out, "     %s(tự động chọn một cổng khả dụng từ: %s)%s\n\n", ColorDim, strings.Join(portsStr, ", "), ColorReset)
	fmt.Fprintf(out, "  %s2.%s Chạy lệnh: %santigravity-telegram-remote start%s\n\n", ColorBold, ColorReset, ColorGreen, ColorReset)
	fmt.Fprintf(out, "  %s3.%s Mở Telegram và gửi tin nhắn cho bot của bạn!\n\n", ColorBold, ColorReset)

	return nil
}

// SetupAction runs the interactive setup wizard with default os.Stdin and os.Stdout.
func SetupAction(ctx context.Context) error {
	return SetupActionWithIO(ctx, os.Stdin, os.Stdout, config.Save)
}
