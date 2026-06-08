ARG BASE_IMAGE=docker.io/alpine

################################################
########### Build the manager binary ###########
################################################
# Build the manager binary
FROM golang:1.26.4-alpine3.22@sha256:727cfc3c40be55cd1bc9a4a059406b28a059857e3be752aa9d09531e12c20c56 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG APPLICATION_VERSION=dev

RUN apk add --no-cache git
WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o generate-config \
    -ldflags "-X 'github.com/scality/virtualip-manager/cmd/config.ApplicationVersion=${APPLICATION_VERSION}'" \
    cmd/main.go

######################################################
########### Build the keepalived binary ##############
######################################################
# NOTE: We need to build keepalived ourself to enable JSON, so that we can
# use the JSON signal to get the current keepalived status in JSON format
# The keepalived binary from the package is not build with JSON enabled
FROM ${BASE_IMAGE} AS build-step

# Keepalived version to use
ARG KEEPALIVED_VERSION

WORKDIR /home/keepalived

RUN apk add --no-cache make gcc curl autoconf automake musl-dev libnl3-dev libnftnl-dev openssl-dev \
  && curl --fail -Lo keepalived.tar.gz https://github.com/acassen/keepalived/archive/refs/tags/v${KEEPALIVED_VERSION}.tar.gz \
  && tar xvf keepalived.tar.gz && cd "keepalived-${KEEPALIVED_VERSION}" \
  && ./autogen.sh \
  && ./configure --enable-vrrp -enable-sha1 --enable-json \
  && make && cp bin/keepalived /keepalived

################################################
########### Build the final image ##############
################################################
FROM ${BASE_IMAGE}

# Timestamp of the build, formatted as RFC3339
ARG BUILD_DATE
# Git revision of the tree at build time
ARG VCS_REF
# Version of the image
ARG VERSION

RUN addgroup -S keepalived \
  && adduser -D -S -G keepalived keepalived \
  && chown -R keepalived:keepalived /run \
  && mkdir -p /etc/keepalived \
  && chown -R keepalived:keepalived /etc/keepalived

COPY --chown=keepalived:keepalived scripts/check-get.sh /etc/keepalived/

COPY --chown=keepalived:keepalived --from=builder /workspace/generate-config /
COPY --chown=keepalived:keepalived scripts/entrypoint.sh /

COPY --chown=keepalived:keepalived --from=build-step /keepalived /usr/sbin/

RUN apk add --no-cache libcap \
  && setcap     cap_net_admin,cap_net_bind_service,cap_net_raw,cap_setuid,cap_setgid=+ep /usr/sbin/keepalived \
  && setcap -v  cap_net_admin,cap_net_bind_service,cap_net_raw,cap_setuid,cap_setgid=+ep /usr/sbin/keepalived \
  && apk del libcap

RUN apk add --no-cache libnl3 libnftnl bash curl

USER keepalived

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/etc/keepalived/keepalived-input.yaml"]

# These contain BUILD_DATE so should come 'late' for layer caching
LABEL maintainer="squad-metalk8s@scality.com" \
      # http://label-schema.org/rc1/
      org.label-schema.build-date="$BUILD_DATE" \
      org.label-schema.name="virtualip-manager" \
      org.label-schema.description="VirtualIP Manager container" \
      org.label-schema.url="https://github.com/scality/virtualip-manager/" \
      org.label-schema.vcs-url="https://github.com/scality/virtualip-manager.git" \
      org.label-schema.vcs-ref="$VCS_REF" \
      org.label-schema.vendor="Scality" \
      org.label-schema.version="$VERSION" \
      org.label-schema.schema-version="1.0" \
      # https://github.com/opencontainers/image-spec/blob/master/annotations.md
      org.opencontainers.image.created="$BUILD_DATE" \
      org.opencontainers.image.authors="squad-virtualip-manager@scality.com" \
      org.opencontainers.image.url="https://github.com/scality/virtualip-manager/" \
      org.opencontainers.image.source="https://github.com/scality/virtualip-manager.git" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.revision="$VCS_REF" \
      org.opencontainers.image.vendor="Scality" \
      org.opencontainers.image.title="virtualip-manager" \
      org.opencontainers.image.description="VirtualIP Manager container" \
      # https://docs.openshift.org/latest/creating_images/metadata.html
      io.openshift.tags="virtualip-manager" \
      io.k8s.description="VirtualIP Manager container" \
      # Various
      com.scality.virtualip-manager.version="$VERSION"