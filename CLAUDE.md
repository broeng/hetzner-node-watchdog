# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Kubernetes controller (`hetzner-node-watchdog`) that watches node `Ready` status and, if a
node stays unavailable too long, restarts the matching Hetzner Cloud server via `hcloud-go`.
Nodes are matched to Hetzner Cloud servers **by name**. See README.md for the full
configuration reference (flags/env vars/defaults) and operational behavior — don't duplicate
it here, but do keep it in sync when changing `internal/config`.

## Commands

```sh
go build .                              # build the binary
go vet ./...                            # static checks
go run . --log-level=debug              # run locally against current kubeconfig context
NODE_WATCHDOG_HCLOUD_TOKEN=... go run . # hcloud token is required; no default
nix build .#hetzner-node-watchdog       # build via flake (mirrors CI/deploy path)
nix build .#chart                       # lint + package the Helm chart (runs `helm lint`)
helm lint deploy --set hcloud.token=x   # lint the chart directly
helm template deploy --set hcloud.token=x  # render chart templates locally
```

There are no test files in the repo currently (`go test ./...` has nothing to run).

## Architecture

`main.go` wires everything together and does nothing else: loads config, builds the k8s
client (in-cluster if `KUBERNETES_SERVICE_HOST` is set, else kubeconfig), builds the hcloud
client, constructs `nodecontroller.Controller` and `health.Controller`, and runs both under a
shared `context` cancelled on SIGTERM/SIGINT.

- **`internal/config`** — a single global config struct (`config.Global`) loaded once via
  `gonfig.Load` (CLI flags > env vars prefixed `NODE_WATCHDOG_` > config file > struct
  defaults). `duration.go` defines a `Duration` type wrapping `time.Duration` with
  `UnmarshalText` so duration fields parse `"5m"`-style strings instead of raw nanoseconds —
  follow this pattern for any new duration-typed field.

- **`internal/nodecontroller`** — the core logic. Central design point: each unavailable node
  gets a single `nextActionAt` timestamp (in `nodeState`, `nodecontroller.go`) that does double
  duty — `TimeoutDuration` seeds it when a node is first observed unavailable, and
  `GracePeriod` resets it after every restart attempt. A k8s informer drives node
  add/update/delete events (`handleNode`); a separate ticker loop (`runRestartLoop`) scans for
  nodes past their `nextActionAt` and restarts them (`Server.Reset`, a hardware reset, not a
  graceful reboot). State is in-memory only (`map[string]*nodeState` + `sync.RWMutex`) — no
  persistence, no leader election, so **only one replica of a given instance should ever run**.
  `hcloud.go` wraps the hcloud client behind small interfaces (`hcloudClienter`,
  `hcloudServerer`, `hcloudActioner`) purely to make it mockable; extend those interfaces
  rather than calling `*hcloud.Client` methods directly from new code in this package.

- **`internal/health`** — a small HTTP server independent of the controller's own logic:
  `/healthz` is unconditional liveness, `/readyz` reflects `nodecontroller.Controller.HasSynced()`
  (the informer's initial sync state), polled every 5s. Any future readiness signal should
  flow through the `ReadinessChecker` interface the same way.

- **`deploy/`** — a Helm chart. Every CLI flag in `internal/config` is mirrored 1:1 under
  `values.yaml`'s `config.*` and rendered as a `--flag=value` arg in `templates/deployment.yaml`
  — when adding a config field, update both. Notably the chart does **not** run a pre-built
  container image: the container's `command`/`args` invoke `nix run <flakeRef> -- <flags>`
  against this repo's own flake, so the pod builds/runs the app straight from source (or a
  pinned ref) on every start. `secondary.*` optionally runs a second Deployment with pod
  anti-affinity against the primary, using a much longer `timeoutDuration`/`gracePeriod` as a
  cheap alternative to leader election (see README's "Redundancy without leader election").

- **`flake.nix`** — defines two Nix packages: `hetzner-node-watchdog` (the Go binary, version
  pinned separately from `go.mod`, injected via `ldflags -X main.version=...`) and `chart`
  (packages `deploy/` into a `.tgz` via `helm lint` + `helm package`). **Three version numbers
  exist independently and must be bumped together when cutting a release**: the `version` in
  `flake.nix`'s `hetzner-node-watchdog` derivation (must match what `main.version` reports),
  the `chart` derivation's own `version` in `flake.nix` (Nix store metadata only), and
  `deploy/Chart.yaml`'s `version`/`appVersion`. Recent commit history bumps these separately —
  check all three when asked to "bump the version".
