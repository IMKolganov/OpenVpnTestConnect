package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	defaultConfigDir   = "./ovpn"
	defaultInterval    = 30 * time.Minute
	connectionTimeout  = 90 * time.Second
	outputTruncateSize = 3000
)

type VPNConfig struct {
	Name     string
	Filename string
	FullPath string
}

type ServerStatus struct {
	Config  VPNConfig
	Success bool
	Output  string
	Error   string
}

func main() {
	// Get environment variables
	ovpnDir := getEnv("VPN_CONFIG_DIR", defaultConfigDir)
	checkInterval := getDurationEnv("CHECK_INTERVAL", defaultInterval)
	botToken := getEnv("TELEGRAM_BOT_TOKEN", "")
	chatID, _ := strconv.ParseInt(getEnv("TELEGRAM_CHAT_ID", ""), 10, 64)

	// Validate required parameters
	if botToken == "" || chatID == 0 {
		fmt.Println("Error: TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must be set")
		return
	}

	// Continuous check loop
	for {
		if err := checkAndReport(ovpnDir, botToken, chatID); err != nil {
			fmt.Printf("Check failed: %v\n", err)
		}
		fmt.Printf("Next check in %s\n", checkInterval)
		time.Sleep(checkInterval)
	}
}

func checkAndReport(ovpnDir, botToken string, chatID int64) error {
	configs, err := getVPNConfigs(ovpnDir)
	if err != nil {
		return fmt.Errorf("error reading configs: %w", err)
	}

	statuses := checkServers(configs)
	sendErrorReport(botToken, chatID, statuses)
	return nil
}

func getVPNConfigs(dir string) ([]VPNConfig, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.ovpn"))
	if err != nil {
		return nil, err
	}

	var configs []VPNConfig
	for _, file := range files {
		configs = append(configs, VPNConfig{
			Name:     strings.TrimSuffix(filepath.Base(file), ".ovpn"),
			Filename: filepath.Base(file),
			FullPath: file,
		})
	}
	return configs, nil
}

func killProcess(process *os.Process) {
	if process == nil {
		return
	}
	process.Kill()
}

func checkServers(configs []VPNConfig) []ServerStatus {
	var statuses []ServerStatus

	for _, config := range configs {
		status := ServerStatus{Config: config}
		fmt.Printf("Checking %s...\n", config.Name)

		ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
		defer cancel()

		// Use system openvpn
		cmd := exec.CommandContext(
			ctx,
			"openvpn",
			"--config", config.FullPath,
			"--auth-nocache",
			"--connect-retry", "1",
			"--connect-retry-max", "1",
			"--verb", "4",
			"--pull-filter", "ignore", "redirect-gateway",
		)

		var outputBuf bytes.Buffer
		cmd.Stdout = &outputBuf
		cmd.Stderr = &outputBuf

		err := cmd.Start()
		if err != nil {
			status.Error = fmt.Sprintf("Failed to start: %v", err)
			statuses = append(statuses, status)
			continue
		}

		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		success := false
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

	loop:
		for {
			select {
			case <-ctx.Done():
				status.Error = "Timeout exceeded (90s)"
				break loop
			case err := <-done:
				if err == nil {
					success = true
				}
				break loop
			case <-ticker.C:
				outputStr := outputBuf.String()
				if strings.Contains(outputStr, "PUSH_REPLY") ||
					strings.Contains(outputStr, "Initialization Sequence Completed") {
					success = true
					cancel()
					break loop
				}
				if strings.Contains(outputStr, "AUTH_FAILED") {
					status.Error = "Authentication failed"
					break loop
				}
				if strings.Contains(outputStr, "TLS Error") {
					status.Error = "TLS handshake failed"
					break loop
				}
				if strings.Contains(outputStr, "RESOLVE") {
					status.Error = "DNS resolution failed"
					break loop
				}
			}
		}

		killProcess(cmd.Process)

		if success {
			status.Success = true
		} else {
			if status.Error == "" {
				status.Error = "Connection failed"
			}
			status.Output = extractRelevantOutput(outputBuf.String())
		}

		statuses = append(statuses, status)
		fmt.Printf("Completed %s: %v\n", config.Name, status.Success)
		time.Sleep(3 * time.Second)
	}

	return statuses
}

func extractRelevantOutput(output string) string {
	if output == "" {
		return "No output captured"
	}
	
	scanner := bufio.NewScanner(strings.NewReader(output))
	var relevant []string
	allLines := []string{}

	for scanner.Scan() {
		line := scanner.Text()
		allLines = append(allLines, line)
		
		if strings.Contains(line, "ERROR") || 
		   strings.Contains(line, "WARNING") || 
		   strings.Contains(line, "VERIFY") || 
		   strings.Contains(line, "AUTH") ||
		   strings.Contains(line, "failed") ||
		   strings.Contains(line, "cannot") ||
		   strings.Contains(line, "unable") ||
		   strings.Contains(line, "TLS Error") ||
		   strings.Contains(line, "RESOLVE") {
			relevant = append(relevant, line)
		}
	}

	if len(relevant) > 0 {
		start := 0
		if len(relevant) > 10 {
			start = len(relevant) - 10
		}
		return strings.Join(relevant[start:], "\n")
	}

	if len(allLines) > 10 {
		allLines = allLines[len(allLines)-10:]
	}
	return strings.Join(allLines, "\n")
}

func sendErrorReport(botToken string, chatID int64, statuses []ServerStatus) {
	var errorReports []string
	total, failed := len(statuses), 0

	for _, status := range statuses {
		if !status.Success {
			failed++
			// Escape backticks for Markdown
			cleanOutput := strings.ReplaceAll(status.Output, "`", "'")
			report := fmt.Sprintf(
				"❌ *%s*\nError: %s\n\n```\n%s\n```",
				status.Config.Name,
				status.Error,
				truncateString(cleanOutput, outputTruncateSize),
			)
			errorReports = append(errorReports, report)
		}
	}

	if len(errorReports) == 0 {
		fmt.Println("All servers OK, no report sent")
		return
	}

	// Final message
	message := fmt.Sprintf(
		"*VPN Error Report*\n\nFailed: %d/%d\n\n%s",
		failed,
		total,
		strings.Join(errorReports, "\n\n"),
	)

	sendTelegramMessage(botToken, chatID, message)
}

func sendTelegramMessage(botToken string, chatID int64, message string) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		fmt.Printf("Telegram error: %v\n", err)
		return
	}

	// Escape special Markdown symbols
	message = escapeMarkdown(message)

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "Markdown"

	if len(message) > 4000 {
		msg.Text = truncateString(message, 4000) + "\n... (truncated)"
	}

	if _, err := bot.Send(msg); err != nil {
		fmt.Printf("Send error: %v\n", err)
	} else {
		fmt.Println("Error report sent successfully")
	}
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		duration, err := time.ParseDuration(value)
		if err == nil {
			return duration
		}
	}
	return defaultValue
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func escapeMarkdown(s string) string {
	// Escape special Markdown characters
	chars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, c := range chars {
		s = strings.ReplaceAll(s, c, "\\"+c)
	}
	return s
}