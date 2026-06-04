# VirtualIP Manager - Design Document

This document describes the internal architecture of `virtualip-manager`. For usage, see the
[README](README.md); for the development workflow, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Overview

`virtualip-manager` is a one-shot CLI (`generate-config`) plus a keepalived runtime, packaged in
a single container. The CLI transforms a declarative VirtualIP spec into a keepalived
configuration file; the container entrypoint then hands off to keepalived.

```
spec.yaml ──> generate-config ──> keepalived.conf ──> keepalived (VRRP)
```

The runtime flow (`scripts/entrypoint.sh`):

1. `generate-config -input <spec> -output /etc/keepalived/keepalived.conf`
2. `exec keepalived --use-file /etc/keepalived/keepalived.conf …`

## Layered architecture

The code follows a clean / hexagonal layout. Dependencies point inward: `domain` depends on
nothing, `service` defines the ports, `usecase` orchestrates them, and `infrastructure` provides
the concrete adapters and wiring.

```
cmd/main.go
    │  parses flags, loads env config, creates DI container
    ▼
pkg/infrastructure/di  ── wires everything (lazy getters)
    │
    ▼
pkg/usecase            ── GenerateConfig.Execute (application logic)
    │  depends only on ports
    ▼
pkg/service            ── ports: ConfigGenerator, InterfaceGetter
    ▲                                  ▲
    │ implemented by                   │ implemented by
pkg/infrastructure/configgenerator   pkg/infrastructure/interfacegetter
    │                                  │
    ▼                                  ▼
pkg/domain             ── VirtualIPConfig, Address, sentinel errors
```

### `pkg/domain`

Pure data and error definitions, no behavior and no outward dependencies:

- `types.go` — `VirtualIPConfig` (apiVersion, kind, addresses, healthcheck) and `Address`
  (`ip`, `node`, `vrId`). The metadata pointer fields (`apiVersion`, `kind`) distinguish "absent"
  from "empty" during validation. Also holds the `EXPECTED_KIND` and `SUPPORTED_API_VERSION`
  validation constants.
- `errors.go` — sentinel errors (`ErrInputFileReading`, `ErrInputFileParsing`,
  `ErrMissingInputParameter`, `ErrInvalidInputParameter`, `ErrTemplating`, `ErrInterfaceNotFound`,
  …) used as wrap targets.

### `pkg/service` (ports)

Interfaces the use case depends on, decoupling it from concrete implementations:

- `ConfigGenerator` — `ParseInputData`, `GenerateConfiguration`.
- `InterfaceGetter` — `GetInterfaceFromIP`.

### `pkg/usecase`

`GenerateConfig.Execute` is the single application flow: read the input file → parse and validate
it (`ParseInputData`) → render the configuration (`GenerateConfiguration`) → write the output (or
print to stdout when no output path is set). File reading and writing are done by the use case
itself; the `ConfigGenerator` port handles only parsing and rendering. It holds the port plus the
configured output path, and wraps every failure with context (`WithDetail` / `WithProperty`).

### `pkg/infrastructure` (adapters)

- `configgenerator/keepalived.go` — the `ConfigGenerator` implementation.
  - `ParseInputData` unmarshals YAML and validates: `kind` must equal `VirtualIPConfiguration`,
    `apiVersion` must be in the supported list, `addresses` must be present and non-empty.
  - `GenerateConfiguration` renders `keepalived.tmpl` (embedded with `//go:embed`). Template
    helpers (`templateFuncs`) expose `add`, `replace`, and `getInterfaceFromIP` (delegated to the
    `InterfaceGetter`). The node identity (`NodeIP`, `NodeName`) is passed in via the template
    data rather than read from the environment by the template; MASTER/BACKUP state and priority
    are decided by comparing each address's `node` to `NodeName`.
- `interfacegetter/hostnetwork.go` — the production `InterfaceGetter`; iterates `net.Interfaces()`
  and returns the interface whose configured subnet contains the target IP.
- `interfacegetter/mock.go` — a static mock used by tests (maps the fixture IPs to `eth0/1/2`).
- `di/` — the `Container`. Each dependency has a lazy getter (`GetConfigGenerator`,
  `GetGenerateConfigUseCase`, `getInterfaceGetter`, `GetMockInterfaceGetter`, `GetLogger`, …)
  that constructs the dependency on first use and caches it. `GetMockInterfaceGetter` lets tests
  swap in the mock adapter without touching production code.

### `cmd`

- `main.go` — parses `-input`/`-output`, loads the environment config, builds the container, and
  runs the use case.
- `config/environment.go` — loads the `Environment` from env vars via `go-envconfig`. `NODE_IP`
  and `NODE_NAME` are required; `LOGGER_LOG_LEVEL` defaults to `info`. Also defines
  `ApplicationName` and `ApplicationVersion` (injected at build time via `-ldflags`).

## Error handling

All errors are built with [`github.com/scality/go-errors`](https://github.com/scality/go-errors):
domain sentinels are the wrap targets, `WithDetail` adds a message, `WithProperty` attaches
structured fields (file paths, expected vs actual values), and `CausedBy` chains the underlying
cause. This keeps failures structured and greppable from the logs.

## Configuration template

`pkg/infrastructure/configgenerator/keepalived.tmpl` is the single source of truth for the
generated config. It emits:

- a `global_defs` block with script security enabled;
- an optional `vrrp_script check_get` block when `healthcheck` is set (probing via
  `/etc/keepalived/check-get.sh`, with `__NODE_IP__` substituted from the `NODE_IP` env var);
- one `vrrp_instance VI_<n>` per address, with `state`, `priority`, `interface` (resolved from
  the IP), `virtual_router_id`, and `virtual_ipaddress`.

## Packaging

The `Dockerfile` is multi-stage:

1. **builder** (golang-alpine) compiles `generate-config` statically (`CGO_ENABLED=0`), injecting
   `ApplicationVersion` via `-ldflags`.
2. **build-step** compiles keepalived from source with `--enable-vrrp --enable-sha1
   --enable-json` — JSON status output is not available in the packaged keepalived.
3. **final** (Alpine) creates a non-root `keepalived` user, copies in the binary, entrypoint, and
   healthcheck script, and uses `setcap` to grant keepalived only the network capabilities it
   needs (`cap_net_admin`, `cap_net_bind_service`, `cap_net_raw`, `cap_setuid`, `cap_setgid`).
