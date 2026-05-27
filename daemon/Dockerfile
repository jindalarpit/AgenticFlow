FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src

# Copy shared module first (dependency)
COPY shared/ ./shared/
COPY daemon/go.mod daemon/go.sum ./daemon/
RUN cd daemon && go mod download

COPY daemon/ ./daemon/

ARG VERSION=dev
ARG COMMIT=unknown
RUN cd daemon && CGO_ENABLED=0 go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o bin/af ./cmd/af

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /src/daemon/bin/af .
EXPOSE 8081
ENTRYPOINT ["./af"]
CMD ["daemon", "start", "--foreground"]
