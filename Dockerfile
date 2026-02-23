FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install CGO dependencies (sqlite3)
RUN apk add --no-cache gcc musl-dev

# Download deps first (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o agent-tools ./cmd/agent-tools

# ---

FROM alpine:3.19

RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app
COPY --from=builder /build/agent-tools .

RUN addgroup -S agenttools && adduser -S agenttools -G agenttools
RUN mkdir -p /data && chown agenttools:agenttools /data
USER agenttools

VOLUME ["/data"]
EXPOSE 8433

ENTRYPOINT ["./agent-tools"]
CMD ["serve", "--addr", ":8433", "--db", "/data/agent-tools.db"]
