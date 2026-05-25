# --- Go build stage ---
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src

# Cache dependencies
COPY server/go.mod server/go.sum ./server/
RUN cd server && go mod download

# Copy server source
COPY server/ ./server/

# Build binaries
ARG VERSION=dev
ARG COMMIT=unknown
RUN cd server && CGO_ENABLED=0 go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o bin/server ./cmd/server
RUN cd server && CGO_ENABLED=0 go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o bin/af ./cmd/af

# --- Runtime stage ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

# Copy Go binaries
COPY --from=builder /src/server/bin/server .
COPY --from=builder /src/server/bin/af .

# Copy migrations
COPY server/migrations/ ./migrations/

EXPOSE 8080

ENTRYPOINT ["./server"]
