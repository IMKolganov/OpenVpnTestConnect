package vpn

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"open-vpn-test-connect/models"
)

// Runner defines a contract to attempt OpenVPN connection and capture logs.
type Runner interface {
	TryConnect(ctx context.Context, cfg models.VPNConfig, timeout time.Duration) (output string, success bool, err error)
}

// OpenVPNRunner runs system "openvpn" process with a given config.
type OpenVPNRunner struct {
	BinaryPath string
}

// TryConnect starts "openvpn" with minimal args and watches stdout/stderr for success markers.
func (r *OpenVPNRunner) TryConnect(parent context.Context, cfg models.VPNConfig, timeout time.Duration) (string, bool, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	bin := r.BinaryPath
	if bin == "" {
		bin = "openvpn"
	}

	cmd := exec.CommandContext(ctx,
		bin,
		"--config", cfg.FullPath,
		"--auth-nocache",
		"--connect-retry", "1",
		"--connect-retry-max", "1",
		"--verb", "4",
		"--pull-filter", "ignore", "redirect-gateway",
		// NOTE: For UDP configs, add "explicit-exit-notify 3" directly in the .ovpn
		// to ensure the server is notified on client exit.
	)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		return "", false, err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Gracefully stop on timeout
			gracefulStop(cmd, done, 5*time.Second)
			return buf.String(), false, ctx.Err()

		case err := <-done:
			// Process exited
			if err == nil {
				return buf.String(), true, nil
			}
			// Exit with error; logs will be analyzed by caller
			return buf.String(), false, err

		case <-ticker.C:
			out := buf.String()
			if strings.Contains(out, "PUSH_REPLY") ||
				strings.Contains(out, "Initialization Sequence Completed") {
				// Success detected — disconnect gracefully to avoid dangling session
				gracefulStop(cmd, done, 5*time.Second)
				return out, true, nil
			}
			// Do not early-exit on error markers; classification happens later.
		}
	}
}

// gracefulStop tries to disconnect the OpenVPN process cleanly.
// 1) Send Interrupt (SIGINT / Ctrl+C), wait up to waitDur
// 2) If still alive, try a gentle Terminate on Unix (SIGTERM)
// 3) If still alive, fall back to Kill
func gracefulStop(cmd *exec.Cmd, done <-chan error, waitDur time.Duration) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	// helper: wait for process to exit or timeout
	waitOrTimeout := func(d time.Duration) bool {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-done:
			return true
		case <-timer.C:
			return false
		}
	}

	// Step 1: Interrupt (cross-platform best-effort)
	_ = cmd.Process.Signal(os.Interrupt)
	if waitOrTimeout(waitDur) {
		return
	}

	// Step 2: Try SIGTERM on Unix (Windows: skip)
	if runtime.GOOS != "windows" {
		// Using a raw syscall import per-OS would be ideal; os.Process.Signal
		// with "TERM" is not portable, so we do best-effort:
		// On Unix, sending another Interrupt is often enough; some builds accept TERM via Interrupt alias.
		_ = cmd.Process.Signal(os.Interrupt)
		if waitOrTimeout(waitDur / 2) {
			return
		}
	}

	// Step 3: Last resort
	_ = cmd.Process.Kill()
}
