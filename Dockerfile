FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY core/ core/
COPY framework/ framework/

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.Version=${VERSION}" \
    -o /optimus-server \
    ./framework/server

FROM alpine:3.21

RUN apk --no-cache add ca-certificates chromium nodejs npm bash \
    && npm install -g @anthropic-ai/claude-code \
    && npm cache clean --force

ENV CHROME_PATH=/usr/bin/chromium-browser

# Pre-configure Claude Code onboarding so it doesn't prompt
RUN echo '{"hasCompletedOnboarding":true}' > /root/.claude.json \
    && mkdir -p /root/.claude

COPY --from=builder /optimus-server /usr/local/bin/optimus-server

EXPOSE 8080

ENTRYPOINT ["optimus-server"]
