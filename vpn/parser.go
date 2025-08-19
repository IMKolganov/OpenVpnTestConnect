package vpn

import (
	"bufio"
	"strings"
)

// ExtractRelevant trims long logs and extracts error-related lines.
func ExtractRelevant(output string, tail int) string {
	if output == "" {
		return "No output captured"
	}

	sc := bufio.NewScanner(strings.NewReader(output))
	var relevant []string
	var all []string

	for sc.Scan() {
		line := sc.Text()
		all = append(all, line)

		l := strings.ToLower(line)
		if strings.Contains(l, "error") ||
			strings.Contains(l, "warning") ||
			strings.Contains(l, "verify") ||
			strings.Contains(l, "auth") ||
			strings.Contains(l, "failed") ||
			strings.Contains(l, "cannot") ||
			strings.Contains(l, "unable") ||
			strings.Contains(l, "tls error") ||
			strings.Contains(l, "resolve") {
			relevant = append(relevant, line)
		}
	}

	if len(relevant) > 0 {
		if len(relevant) > tail {
			relevant = relevant[len(relevant)-tail:]
		}
		return strings.Join(relevant, "\n")
	}

	// Fallback to the last N lines
	if len(all) > tail {
		all = all[len(all)-tail:]
	}
	return strings.Join(all, "\n")
}

// ClassifyError tries to derive a human-readable error from output.
func ClassifyError(output string) string {
	o := strings.ToUpper(output)
	switch {
	case strings.Contains(o, "AUTH_FAILED"):
		return "Authentication failed"
	case strings.Contains(o, "TLS ERROR") || strings.Contains(o, "TLS HANDSHAKE"):
		return "TLS handshake failed"
	case strings.Contains(o, "RESOLVE") || strings.Contains(o, "TRYING TO RESOLVE"):
		return "DNS resolution failed"
	default:
		return "Connection failed"
	}
}
