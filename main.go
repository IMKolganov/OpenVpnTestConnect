package main

import (
	"context"
	"fmt"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"open-vpn-test-connect/app"
	"open-vpn-test-connect/env"
	"open-vpn-test-connect/notify"
	"open-vpn-test-connect/version"
	"open-vpn-test-connect/vpn"
)

const (
	defaultConfigDir  = "./ovpn"
	defaultInterval   = 30 * time.Minute
	connectionTimeout = 90 * time.Second
	outputTailLines   = 10
	telegramHardLimit = 4000
)

func main() {
	// Print version info at startup
	fmt.Printf("OpenVPN Test Connect Monitor\nVersion: %s\nCommit: %s\nBuilt: %s\n\n",
		version.Version, version.Commit, version.BuildDate)

	// Read env
	ovpnDir := env.Get("VPN_CONFIG_DIR", defaultConfigDir)
	checkInterval := env.GetDuration("CHECK_INTERVAL", defaultInterval)
	botToken := env.Get("TELEGRAM_BOT_TOKEN", "")
	chatIDStr := env.Get("TELEGRAM_CHAT_ID", "")
	chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)

	// Validate required
	if botToken == "" || chatID == 0 {
		fmt.Println("Error: TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must be set")
		return
	}

	// Wire dependencies
	runner := &vpn.OpenVPNRunner{BinaryPath: ""} // use PATH "openvpn"
	reporter := &notify.TelegramReporter{
		Token:           botToken,
		ChatID:          chatID,
		OutputTail:      outputTailLines,
		OutputHardLimit: telegramHardLimit,
	}

	application := &app.App{
		ConfigDir:        ovpnDir,
		CheckInterval:    checkInterval,
		TimeoutPerServer: connectionTimeout,
		OutputTail:       outputTailLines,
		OutputLimit:      telegramHardLimit,
		Runner:           runner,
		Reporter:         reporter,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := application.Run(ctx); err != nil && err != context.Canceled {
		fmt.Printf("Stopped with error: %v\n", err)
	}
}
