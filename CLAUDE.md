# virtualip-manager

This is a **Go service that manages VirtualIPs in multinetwork environments**. It is packaged as a single `manager` binary (distroless, non-root) and follows a Kubernetes operator/controller-style layout. It contains:

- Entry point and command wiring (`cmd/`)
- Build-time configuration such as application name and version (`cmd/config/`)
- Containerized build via multi-stage `Dockerfile` (golang-alpine builder → distroless/static runtime)
- CI workflows for build, test, e2e, release, promote, SBOM, and pre/post-merge checks (`.github/workflows/`)

Tech stack: Go 1.25, modules (`go.mod`), `golangci-lint` for linting, Renovate for dependency updates. Expect mostly `.go` files, plus YAML workflows and the `Dockerfile`.
