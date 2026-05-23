# --- Go build stage ---
FROM golang:1.23-alpine AS go-builder

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

# --- Web build stage ---
FROM node:20-alpine AS web-builder

WORKDIR /src/web

# Cache dependencies
COPY web/package.json web/package-lock.json* ./
RUN npm ci --ignore-scripts

# Copy web source and build
COPY web/ ./
RUN npm run build

# --- Runtime stage ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy Go binaries
COPY --from=go-builder /src/server/bin/server .
COPY --from=go-builder /src/server/bin/af .

# Copy migrations
COPY server/migrations/ ./migrations/

# Copy web dist
COPY --from=web-builder /src/web/dist ./web/dist

EXPOSE 8080

ENTRYPOINT ["./server"]
