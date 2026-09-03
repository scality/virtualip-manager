# virtualip-manager

[![Post Merge](https://github.com/scality/virtualip-manager/actions/workflows/post-merge.yaml/badge.svg)](https://github.com/scality/virtualip-manager/actions/workflows/post-merge.yaml)
[![GitHub release](https://img.shields.io/github/v/release/scality/virtualip-manager)](https://github.com/scality/virtualip-manager/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/scality/virtualip-manager)](go.mod)
[![License](https://img.shields.io/github/license/scality/virtualip-manager)](LICENSE)

A manager for VirtualIPs in multinetwork environments.

`virtualip-manager` renders a [keepalived](https://www.keepalived.org/) configuration from a
declarative VirtualIP spec and runs keepalived to advertise those VIPs over VRRP. It is
distributed as a container image that bundles the `generate-config` binary together with a
purpose-built keepalived (compiled with JSON status support).

## How it works

The container entrypoint runs two steps:

1. `generate-config -input <spec.yaml> -output /etc/keepalived/keepalived.conf` — reads and
   validates the VirtualIP spec, resolves each address to a host network interface, and renders
   the keepalived config.
2. `exec keepalived …` — starts keepalived against the generated config.

For each address, the node whose `NODE_NAME` matches the address's `node` becomes the VRRP
`MASTER` (priority 150); every other node is a `BACKUP` (priority 100).

## Input spec

The input is a `VirtualIPConfiguration` document (apiVersion `loadbalancer.scality.com/v1alpha1`):

```yaml
---
apiVersion: loadbalancer.scality.com/v1alpha1
kind: VirtualIPConfiguration
addresses:
  - ip: 172.18.0.10      # virtual IP to advertise
    node: bootstrap      # node that should own this VIP (becomes MASTER)
    vrId: 51            # VRRP virtual_router_id (must be unique per VIP on the segment)
  - ip: 172.18.0.11
    node: node1
    vrId: 52
healthcheck: https://__NODE_IP__:443/healthz   # optional; __NODE_IP__ is substituted at runtime
```

- `addresses` is required and must be non-empty. Each entry needs `ip`, `node`, and `vrId`.
- `healthcheck` is optional. When set, a `vrrp_script` is added that probes the URL every 5s via
  `/etc/keepalived/check-get.sh` (shipped from `scripts/check-get.sh`); the `__NODE_IP__` token is
  replaced with the `NODE_IP` env var.

## Check script

The check script is used by keepalived to check that the local node, where the keepalived process is running, is ready to get some load.

## Configuration

`generate-config` is configured by environment variables (validated at startup):

| Variable           | Required | Default | Purpose                                            |
| ------------------ | -------- | ------- | -------------------------------------------------- |
| `NODE_IP`          | yes      | —       | This node's IP; substituted into the healthcheck.  |
| `NODE_NAME`        | yes      | —       | This node's name; decides MASTER vs BACKUP.        |
| `LOGGER_LOG_LEVEL` | no       | `info`  | Log level for the structured (slog) logger.        |

Flags: `-input <path>` (required) and `-output <path>` (defaults to stdout).

## Usage

Run locally against a spec file:

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

Build and run the container:

```sh
make docker-build                 # builds virtualip-manager:latest
docker run --rm --privileged --network host \
  -e NODE_NAME=bootstrap -e NODE_IP=172.18.0.1 \
  -v "$PWD/spec.yaml:/etc/keepalived/keepalived-input.yaml" \
  virtualip-manager:latest
```

## Documentation

- [DESIGN.md](DESIGN.md) — architecture and internals.
- [CONTRIBUTING.md](CONTRIBUTING.md) — development workflow and conventions.
