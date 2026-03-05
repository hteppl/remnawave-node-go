FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG XRAY_VERSION=v26.2.6

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags "-X github.com/remnawave/node-go/internal/version.Version=${VERSION} -X github.com/remnawave/node-go/internal/version.BuildTime=${BUILD_TIME} -w -s" \
    -o remnawave-node-go ./cmd/node-go

FROM alpine:3.21

RUN apk add --no-cache ca-certificates curl tzdata unzip

WORKDIR /app

COPY --from=builder /app/remnawave-node-go .
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV XRAY_VERSION=${XRAY_VERSION}
ENV XRAY_LOCATION_ASSET=/usr/local/share/xray

EXPOSE 2222

ENTRYPOINT ["/entrypoint.sh"]
CMD ["./remnawave-node-go"]
