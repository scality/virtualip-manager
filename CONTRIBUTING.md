# Contributing

Thanks for contributing to `virtualip-manager`. This guide covers the local development workflow,
project conventions, and what CI expects. For the architecture, see [DESIGN.md](DESIGN.md).

## Prerequisites

- Go (the version is pinned in [`go.mod`](go.mod)).
- Docker (or another `CONTAINER_TOOL`, e.g. podman) for building the image.
- `golangci-lint` is installed automatically into `bin/` by `make lint`.
- [Kind](https://kind.sigs.k8s.io/) and `kubectl`, for `make test-e2e` only (install Kind manually;
  override the binary with `KIND=…`).
- Git for version control

Everyday commands:

```sh
make test                # unit + integration tests
make test-e2e            # end-to-end tests on a Kind cluster
make lint                # golangci-lint
make docker-build        # build the container image
```

## Architecture
The project uses a clean-architecture layering with an inward-only dependency rule (cmd → infrastructure → usecase → service → domain). Before adding a type, read [DESIGN.md](DESIGN.md) and place it in the layer that owns its responsibility:

entities go in domain (no external dependencies);
a new capability the use cases consume is a small port in service;
filesystem / OS code is an adapter in infrastructure;
application logic is a use case, never an adapter.

See [DESIGN.md](DESIGN.md) for how these layers fit together.

## Coding conventions

- **Logging**: standard-library `log/slog`.
- **Errors**: `github.com/scality/go-errors`, imported unaliased as `errors`.
  Define package-level `ErrXxx` sentinels and wrap at the failure site with
  `errors.Wrap(ErrXxx, errors.WithDetail(...), errors.WithProperty(...),
  errors.CausedBy(rawErr))` so `errors.Is` keeps matching the category.
- **Tests**: Use Ginkgo + Gomega.
- **Linting & formatting**: `golangci-lint run` must pass.

## Run the CLI locally

```sh
cat >spec.yaml <<EOF
---
apiVersion: loadbalancer.scality.com/v1alpha1
kind: VirtualIPConfiguration
addresses:
- ip: 172.17.0.15
  node: bootstrap
  vrId: 51
- ip: 172.17.0.16
  node: node1
  vrId: 52
- ip: 172.17.0.17
  node: node2
  vrId: 53
healthcheck: https://__NODE_IP__:443/healthz
EOF
NODE_NAME=bootstrap NODE_IP=1.1.1.1 \
  go run ./cmd -input ./spec.yaml
```

## Keep the docs in sync

Treat the docs as part of the change, not an afterthought. In the same PR:

- a change to behavior, flags, or output → update the [README](README.md);
- a change to architecture or a design decision → update
  [DESIGN.md](DESIGN.md);
- a change to conventions or workflow → update this file.

## Commits & pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, …).
- Keep commits focused on a single logical change.
- Keep PRs focused and reasonably sized, with a clear description and linked issues.
- Ensure CI passes before requesting review. The pre-merge workflow runs the build, `go generate`
  drift check, `golangci-lint`, unit tests, e2e tests, and SBOM generation.

## License

By contributing you agree your contribution is licensed under the repository's [LICENSE](LICENSE)
