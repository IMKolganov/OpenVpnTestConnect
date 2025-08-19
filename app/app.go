package app

import (
	"context"
	"fmt"
	"time"

	"open-vpn-test-connect/models"
	"open-vpn-test-connect/notify"
	"open-vpn-test-connect/util"
	"open-vpn-test-connect/vpn"
)

// App encapsulates dependencies and main loop.
type App struct {
	ConfigDir        string
	CheckInterval    time.Duration
	TimeoutPerServer time.Duration
	OutputTail       int
	OutputLimit      int

	Runner   vpn.Runner
	Reporter notify.Reporter
}

// Run starts the periodic checking loop (blocking).
func (a *App) Run(ctx context.Context) error {
	ticker := time.NewTicker(a.CheckInterval)
	defer ticker.Stop()

	// Initial immediate run
	if err := a.once(ctx); err != nil {
		fmt.Printf("Check failed: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := a.once(ctx); err != nil {
				fmt.Printf("Check failed: %v\n", err)
			}
		}
	}
}

func (a *App) once(ctx context.Context) error {
	cfgs, err := util.DiscoverConfigs(a.ConfigDir)
	if err != nil {
		return fmt.Errorf("error reading configs: %w", err)
	}

	var statuses []models.ServerStatus
	for _, c := range cfgs {
		fmt.Printf("Checking %s...\n", c.Name)

		out, ok, runErr := a.Runner.TryConnect(ctx, c, a.TimeoutPerServer)
		st := models.ServerStatus{
			Config:  c,
			Success: ok,
		}

		if ok {
			fmt.Printf("Completed %s: success\n", c.Name)
		} else {
			if runErr != nil && st.Error == "" {
				// If process returned error, still classify based on logs
				_ = runErr
			}
			st.Output = vpn.ExtractRelevant(out, a.OutputTail)
			st.Error = vpn.ClassifyError(out)
			fmt.Printf("Completed %s: failed (%s)\n", c.Name, st.Error)
		}

		statuses = append(statuses, st)
		// Avoid hammering multiple OpenVPN processes back-to-back on some OS
		time.Sleep(2 * time.Second)
	}

	// Send a single consolidated report (only failures will be included)
	if err := a.Reporter.SendReport(statuses); err != nil {
		fmt.Printf("Telegram send error: %v\n", err)
	}
	return nil
}
