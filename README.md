# VPN Config Checker

A minimal Go-based tool that checks the connectivity of OpenVPN `.ovpn` configuration files and sends error reports via Telegram.

## Features

- Automatically scans all `.ovpn` files in a specified directory
- Connects to each VPN configuration using OpenVPN CLI
- Detects and logs various connection issues (TLS errors, auth failures, etc.)
- Sends summarized error reports to a Telegram chat
- Fully dockerized and isolated VPN environment
- Suitable for CI, cron jobs, or server health monitoring

## Requirements

- Docker (recommended)
- Or: Go 1.24+ and system-installed `openvpn`
- A Telegram Bot Token
- A Telegram Chat ID (your personal ID or a group ID)

## Environment Variables

| Variable              | Description                                         |
|-----------------------|-----------------------------------------------------|
| `VPN_CONFIG_DIR`      | Path to directory containing `.ovpn` files (default: `./ovpn`) |
| `CHECK_INTERVAL`      | How often to run the checks (e.g. `30m`, `1h`)      |
| `TELEGRAM_BOT_TOKEN`  | Telegram bot token from @BotFather                  |
| `TELEGRAM_CHAT_ID`    | Chat ID to send error reports to                    |

## Usage

### With Docker Compose

```bash
docker compose up --build -d
```

`.env` file example:

```
TELEGRAM_BOT_TOKEN=123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11
TELEGRAM_CHAT_ID=123456789
```

### With Docker CLI

```bash
docker run --rm \
  --cap-add=NET_ADMIN \
  --device=/dev/net/tun \
  -e TELEGRAM_BOT_TOKEN=your_token \
  -e TELEGRAM_CHAT_ID=123456789 \
  -v $(pwd)/ovpn:/app/ovpn:ro \
  vpnchecker
```

## Building Locally

```bash
go build -o vpnchecker
./vpnchecker
```

## Output Format

The bot will send messages like:

```
*VPN Error Report*

Failed: 2/4

❌ *Cyprus TCP*
Error: Connection failed

\`\`\`
TLS handshake failed
RESOLVE: DNS resolution failed
...
\`\`\`
```

## Notes

- Make sure `.ovpn` files have proper inline certs and Unix line endings.
- Docker container must have access to `/dev/net/tun` and `NET_ADMIN` capability.
- Telegram bots must have access to your chat. Send a message first to initialize.

## License

MIT License