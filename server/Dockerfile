FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src

# Copy shared module first (dependency)
COPY shared/ ./shared/
COPY server/go.mod server/go.sum ./server/
RUN cd server && go mod download

COPY server/ ./server/

ARG VERSION=dev
ARG COMMIT=unknown
RUN cd server && CGO_ENABLED=0 go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o bin/server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=builder /src/server/bin/server .
COPY server/migrations/ ./migrations/
EXPOSE 8080
ENTRYPOINT ["./server"]
