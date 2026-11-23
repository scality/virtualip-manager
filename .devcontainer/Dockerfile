FROM mcr.microsoft.com/devcontainers/go:1.27-trixie@sha256:74f45fc4c2abe0de9a9acdb4225ffb3e58e6b28f9af2e209afaa8920a3d3b623
# The devcontainers image tag has no Go patch version, and the image sets
# GOTOOLCHAIN=local. Take Go from the Dockerfile's golang image, so the
# patch is pinned and matches go.mod.
RUN rm -rf /usr/local/go
COPY --from=golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 /usr/local/go /usr/local/go

USER vscode

# renovate: datasource=github-releases depName=golangci/golangci-lint extractVersion=^v(?<version>.*)$
ARG GOLANGCI_LINT_VERSION=2.15.0

RUN curl -sSfL  https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sudo sh -s -- v${GOLANGCI_LINT_VERSION}

# renovate: datasource=github-releases depName=kubernetes-sigs/kind
ARG KIND_VERSION=v0.33.0

RUN curl -sSfLO https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-linux-amd64 && \
    chmod +x kind-linux-amd64 && \
    sudo mv kind-linux-amd64 /usr/local/bin/kind

# renovate: datasource=github-releases depName=kubernetes/kubernetes
ARG KUBECTL_VERSION=v1.37.2

RUN curl -sSfLO https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    chmod +x kubectl && \
    sudo mv kubectl /usr/local/bin/kubectl
