FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o vpn-checker .

FROM alpine:latest
RUN apk add --no-cache openvpn
WORKDIR /app
COPY --from=builder /app/vpn-checker /app/
COPY --from=builder /app/ovpn /app/ovpn

ENV VPN_CONFIG_DIR=/app/ovpn
ENV CHECK_INTERVAL=30m
ENV TELEGRAM_BOT_TOKEN=""
ENV TELEGRAM_CHAT_ID=""

CMD ["/app/vpn-checker"]