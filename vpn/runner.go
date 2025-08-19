package vpn

import (
	"bytes"
	"context"
	"os"
	"os/exec"
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
	)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	// Start process
	if err := cmd.Start(); err != nil {
		return "", false, err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Watch for success / error markers in output
	for {
		select {
		case <-ctx.Done():
			killProc(cmd.Process)
			return buf.String(), false, ctx.Err()
		case err := <-done:
			// Process exited — if no error, treat as success (rare),
			// otherwise analyze logs for error.
			if err == nil {
				return buf.String(), true, nil
			}
			return buf.String(), false, err
		case <-ticker.C:
			out := buf.String()
			if strings.Contains(out, "PUSH_REPLY") ||
				strings.Contains(out, "Initialization Sequence Completed") {
				// Success path: stop quickly
				killProc(cmd.Process)
				return out, true, nil
			}
			// We do not early-exit on error markers: let timeout/exit decide,
			// classification happens later based on logs.
		}
	}
}

func killProc(p *os.Process) {
	if p != nil {
		_ = p.Kill()
	}
}
