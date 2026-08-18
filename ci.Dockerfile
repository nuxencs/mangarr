# build base
FROM --platform=$BUILDPLATFORM golang:1.26.6-alpine3.23 AS app-base

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
    -X mangarr/internal/buildinfo.Version=${VERSION} \
    -X mangarr/internal/buildinfo.Commit=${REVISION} \
    -X mangarr/internal/buildinfo.Date=${BUILDTIME}" \
    -o /out/bin/mangarr main.go

# build runner
FROM alpine:3.23 AS runner
RUN apk add --no-cache ca-certificates tini tzdata

LABEL org.opencontainers.image.source="https://github.com/nuxencs/mangarr" \
    org.opencontainers.image.licenses="MIT" \
    org.opencontainers.image.base.name="alpine:3.23"

ENV HOME="/config" \
    XDG_CONFIG_HOME="/config" \
    XDG_DATA_HOME="/config"

COPY --link --from=mangarr /out/bin/mangarr /usr/bin/

USER nobody:nogroup
WORKDIR /config
VOLUME ["/config"]

ENTRYPOINT ["/sbin/tini", "--"]

CMD ["/usr/bin/mangarr", "monitor", "--config", "/config"]
