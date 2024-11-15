# build base
FROM --platform=$BUILDPLATFORM golang:1.23-alpine3.20 AS app-base

WORKDIR /src

ENV SERVICE=mangarr
ARG VERSION=dev \
    REVISION=dev \
    BUILDTIME \
    TARGETOS TARGETARCH TARGETVARIANT

COPY go.mod go.sum ./
RUN go mod download
COPY . ./

# build mangarr
FROM --platform=$BUILDPLATFORM app-base AS mangarr
RUN --network=none --mount=target=. \
    export GOOS=$TARGETOS; \
    export GOARCH=$TARGETARCH; \
    [[ "$GOARCH" == "amd64" ]] && export GOAMD64=$TARGETVARIANT; \
    [[ "$GOARCH" == "arm" ]] && [[ "$TARGETVARIANT" == "v6" ]] && export GOARM=6; \
    [[ "$GOARCH" == "arm" ]] && [[ "$TARGETVARIANT" == "v7" ]] && export GOARM=7; \
    echo $GOARCH $GOOS $GOARM$GOAMD64; \
    go build -ldflags "-s -w \
    -X buildinfo.Version=${VERSION} \
    -X buildinfo.Commit=${REVISION} \
    -X buildinfo.Date=${BUILDTIME}" \
    -o /out/bin/mangarr main.go

# build runner
FROM alpine:latest as RUNNER
RUN apk add --no-cache ca-certificates curl tzdata jq

LABEL org.opencontainers.image.source = "https://github.com/nuxencs/mangarr" \
      org.opencontainers.image.licenses = "MIT" \
      org.opencontainers.image.base.name = "alpine:latest"

ENV HOME="/config" \
    XDG_CONFIG_HOME="/config" \
    XDG_DATA_HOME="/config"

WORKDIR /app
VOLUME /config

COPY --link --from=mangarr /out/bin/mangarr /usr/bin/

ENTRYPOINT ["/usr/bin/mangarr", "monitor", "--config", "/config"]