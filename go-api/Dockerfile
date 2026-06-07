# ── Stage 1: Build ───────────────────────────────────────────
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Set GONOSUMDB so go can download without a pre-existing go.sum
ENV GONOSUMDB='*'

# Copy module definition first (cache layer for dependencies)
COPY go.mod ./

# Download all dependencies declared in go.mod
RUN go mod download

# Copy source code
COPY *.go ./

# Build a statically-linked binary (no CGO needed for pure Go code)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server .

# ── Stage 2: Minimal runtime image ───────────────────────────
FROM alpine:3.19

# CA certificates are needed for HTTPS outbound calls
RUN apk add --no-cache ca-certificates wget

WORKDIR /app

COPY --from=builder /app/server .

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:3000/health || exit 1

CMD ["./server"]
