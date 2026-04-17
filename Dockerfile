# ── Stage 1: build ──────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go test ./... && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o luhn-generator .

# ── Stage 2: minimal runtime image (~5 MB) ───────────────────────────────────
FROM scratch

COPY --from=builder /app/luhn-generator /luhn-generator

EXPOSE 8080

ENTRYPOINT ["/luhn-generator"]
