FROM golang:1.14-alpine as builder

WORKDIR /workspace

# copy modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache modules
RUN go mod download

# copy source code
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/

# build
RUN CGO_ENABLED=0 go build -a -o notification-controller main.go

FROM alpine:3.12

ARG BUILD_DATE
ARG VERSION
ARG REVISION

LABEL org.opencontainers.image.created=$BUILD_DATE \
  org.opencontainers.image.version=$VERSION \
  org.opencontainers.image.revision=$REVISION \
  org.opencontainers.image.licenses="Apache-2.0" \
  org.opencontainers.image.source="https://github.com/fluxcd/notification-controller" \
  org.opencontainers.image.documentation="https://toolkit.fluxcd.io" \
  org.opencontainers.image.authors="Flux CD"

RUN apk add --no-cache ca-certificates tini

COPY --from=builder /workspace/notification-controller /usr/local/bin/

# Create minimal nsswitch.conf file to prioritize the usage of /etc/hosts over DNS queries.
# https://github.com/gliderlabs/docker-alpine/issues/367#issuecomment-354316460
RUN [ ! -e /etc/nsswitch.conf ] && echo 'hosts: files dns' > /etc/nsswitch.conf

RUN addgroup -S controller && adduser -S -g controller controller

USER controller

ENTRYPOINT [ "/sbin/tini", "--", "notification-controller" ]
