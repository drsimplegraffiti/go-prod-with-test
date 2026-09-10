# --- Build stage ---
FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/goapi ./cmd/api

# --- Runtime stage ---
FROM alpine:3.20

RUN apk add --no-cache ca-certificates && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /out/goapi /app/goapi

USER app
EXPOSE 8080

ENTRYPOINT ["/app/goapi"]
