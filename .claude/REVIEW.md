# Review criteria

Read by the `/review-pr` skill (Scality agent hub) and by anyone reviewing by hand.
Flag problems only — see "What not to flag" at the end.

## What this repo is

`virtualip-manager` is a Go service that manages VirtualIPs in multinetwork
environments: it assigns and releases addresses on node interfaces through netlink
and keeps that state converged.

## Criteria

| Area | What to check |
|------|---------------|
| Error wrapping | Use `fmt.Errorf("...: %w", err)` to wrap errors (not `%v`); don't swallow errors or ignore returned `err` values |
| Context propagation | Pass `context.Context` through call chains as the first argument; respect cancellation and deadlines; don't store contexts in structs |
| Goroutine leaks | Every goroutine must have a clear exit condition; prefer `errgroup`/`sync.WaitGroup`; avoid leaking goroutines on error paths |
| Networking / VirtualIP logic | Correct handling of IPs, interfaces, and netlink operations; idempotent add/remove of VIPs; cleanup of assigned addresses on shutdown or failure |
| Kubernetes / reconciler patterns | Reconciler idempotency, RBAC scoping, status subresource updates, proper requeue on transient errors (if controller-runtime is used) |
| Interface compliance | Assert implementations satisfy interfaces at compile time (`var _ Iface = (*Impl)(nil)`) |
| Concurrency safety | Guard shared state with mutexes or channels; check for data races on maps and shared counters |
| Resource cleanup | `defer` close/unlock for files, connections, and locks; no leaked file descriptors or sockets |
| Config & versioning | Build-time vars (e.g. `ApplicationVersion` via ldflags) stay consistent; env var naming and defaults are backward compatible |
| Dependencies | `go.mod`/`go.sum` changes are tidy and consistent; pin module versions, no unintended major bumps |
| Security | No hardcoded credentials or secrets; validate external input; least-privilege for network and Kubernetes operations |
| Breaking changes | Anything that changes public Go APIs, exported types, CLI flags, or container interfaces |

## What not to flag

- Anything the linters already own: `golangci-lint`, `gofmt` — formatting, import
  order, unused variables, naming.
- Markdown or comment wording preferences.
- Refactors unrelated to the PR's purpose.
