# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o api ./cmd/api

FROM alpine:3.20
# chromium + noto-cjk are required for the PDF receipt renderer. The chromedp
# package shells out to whichever Chromium binary it finds on PATH; without
# this, GenerateForOrder fails with "exec: chromium-browser: executable file
# not found in $PATH". The noto-cjk fonts cover Traditional Chinese receipt
# labels (zh-Hant locale). font-noto-emoji rounds out the glyph coverage for
# any address fields containing emojis from copy-paste.
RUN apk add --no-cache \
        ca-certificates tzdata libwebp-tools ffmpeg \
        chromium nss freetype harfbuzz ttf-freefont \
        font-noto-cjk font-noto-emoji
ENV CHROME_BIN=/usr/bin/chromium-browser
WORKDIR /app
COPY --from=builder /app/api .
EXPOSE 8080
CMD ["./api"]
