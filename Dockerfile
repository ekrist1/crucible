# Crucible Monitor Docker Image
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o crucible-monitor .

FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Create non-root user
RUN addgroup -g 1001 -S crucible && \
    adduser -u 1001 -S crucible -G crucible

# Copy binary and configs
COPY --from=builder /app/crucible-monitor .
COPY --from=builder /app/configs ./configs

# Create data directory
RUN mkdir -p /data && chown crucible:crucible /data

USER crucible

EXPOSE 9090

# Environment variables (override with docker run -e)
ENV RESEND_API_KEY=""
ENV ALERT_FROM_EMAIL=""
ENV ALERT_FROM_NAME="Crucible Monitor"

CMD ["./crucible-monitor"]