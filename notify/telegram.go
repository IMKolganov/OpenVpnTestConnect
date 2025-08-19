package notify

import (
	"fmt"
	"strings"

	"open-vpn-test-connect/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Reporter interface {
	SendReport(statuses []models.ServerStatus) error
}

type TelegramReporter struct {
	Token           string
	ChatID          int64
	OutputTail      int
	OutputHardLimit int // hard limit per message (Telegram 4096 is a safe guideline)
}

// SendReport builds a markdown message and sends it to Telegram.
func (r *TelegramReporter) SendReport(statuses []models.ServerStatus) error {
	var blocks []string
	total := len(statuses)
	failed := 0

	for _, st := range statuses {
		if st.Success {
			continue
		}
		failed++

		out := strings.ReplaceAll(st.Output, "`", "'")
		block := fmt.Sprintf(
			"❌ *%s*\nError: %s\n\n```\n%s\n```",
			escapeMd(st.Config.Name),
			escapeMd(st.Error),
			truncate(out, r.OutputHardLimit),
		)
		blocks = append(blocks, block)
	}

	// Nothing to send if all OK
	if len(blocks) == 0 {
		return nil
	}

	msgText := fmt.Sprintf("*VPN Error Report*\n\nFailed: %d/%d\n\n%s",
		failed, total, strings.Join(blocks, "\n\n"))

	bot, err := tgbotapi.NewBotAPI(r.Token)
	if err != nil {
		return err
	}

	msg := tgbotapi.NewMessage(r.ChatID, msgText)
	msg.ParseMode = "Markdown"
	if len(msg.Text) > r.OutputHardLimit {
		msg.Text = truncate(msg.Text, r.OutputHardLimit-30) + "\n... (truncated)"
	}

	_, err = bot.Send(msg)
	return err
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func escapeMd(s string) string {
	// Telegram Markdown escapings
	r := s
	for _, c := range []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"} {
		r = strings.ReplaceAll(r, c, "\\"+c)
	}
	return r
}
