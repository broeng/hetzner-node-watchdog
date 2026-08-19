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
go test ./...                           # run tests (currently: internal/config only)
go test ./internal/config/... -run TestLoadDefaults -v  # run a single test
go run . --log-level=debug              # run locally against current kubeconfig context
NODE_WATCHDOG_HCLOUD_TOKEN=... go run . # hcloud token is required; no default
nix build .#hetzner-node-watchdog       # build via flake (mirrors CI/deploy path)
nix build .#chart                       # lint + package the Helm chart (runs `helm lint`)
helm lint deploy --set hcloud.token=x   # lint the chart directly
helm template deploy --set hcloud.token=x  # render chart templates locally
```

`internal/config`'s tests call `Load()` directly, which reads `os.Args`/env vars — they
replace `os.Args` for the duration of each test (see `withArgs` in `config_test.go`) rather
than relying on `go test`'s own flags not tripping gonfig's flag parser (which errors on any
argument it doesn't recognize). Follow that pattern for any new test that calls `Load()`.
`internal/nodecontroller` and `internal/health` have no tests yet.

## Architecture

`main.go` wires everything together and does nothing else: calls `config.Load()`, builds the
k8s client (in-cluster if `KUBERNETES_SERVICE_HOST` is set, else kubeconfig), builds the hcloud
client, constructs `nodecontroller.Controller` and `health.Controller`, and runs both under a
shared `context` cancelled on SIGTERM/SIGINT. `main` never touches CLI-flag/env-var details
directly — it only ever sees `*config.Configuration`.

- **`internal/config`** — the only package that knows about gonfig/CLI flags/env vars; that
  raw, string-typed shape (`rawFlags`, unexported, local to `Load()`) never leaves the package.
  `Load()` parses it (CLI flags > env vars prefixed `NODE_WATCHDOG_` > config file > struct
  defaults, via `gonfig.Load`), validates it (log level, required hcloud token, selector
  syntax), and returns the exported `*Configuration` — fully-typed (`time.Duration`,
  `logrus.Level`, `[]labels.Selector`, `[]TaintSelector`) and ready to use, which is the only
  thing the rest of the app depends on. `duration.go` defines an unexported `duration` type
  wrapping `time.Duration` with `UnmarshalText` so gonfig parses `"5m"`-style strings instead of
  raw nanoseconds — follow this pattern for any new duration-typed raw field.
  `selectors.go` parses the `--ignore-node-label-selector`/`--ignore-node-taint-selector` flags
  (each a comma-separated list, OR'd together; quote an entry to embed a literal comma) into
  `[]labels.Selector` and `[]TaintSelector` respectively — `TaintSelector` mirrors
  `kubectl taint`'s `key[=value][:effect]` syntax, with an omitted value/effect acting as a
  wildcard. If you add another selector-shaped flag, parse it here too, not in the consuming
  package.

- **`internal/nodecontroller`** — the core logic. Consumes a `*config.Configuration` (passed
  into `New` and stored on `Controller`) — it never parses config itself, only applies the
  already-parsed values. Central design point: each unavailable node gets a single
  `nextActionAt` timestamp (in `nodeState`, `nodecontroller.go`) that does double duty —
  `TimeoutDuration` seeds it when a node is first observed unavailable, and `GracePeriod`
  resets it after every restart attempt. A k8s informer drives node add/update/delete events
  (`handleNode`); a separate ticker loop (`runRestartLoop`) scans for nodes past their
  `nextActionAt` and restarts them (`Server.Reset`, a hardware reset, not a graceful reboot).
  Before tracking a node, `nodeIsIgnored` checks static "don't touch this node" criteria —
  cordoned (if `IgnoreCordoned`), or matching an ignore label/taint selector — independent of
  the node's actual readiness; matching clears any existing tracked state too. State is
  in-memory only (`map[string]*nodeState` + `sync.RWMutex`) — no persistence, no leader
  election, so **only one replica of a given instance should ever run**. `hcloud.go` wraps the
  hcloud client behind small interfaces (`hcloudClienter`, `hcloudServerer`, `hcloudActioner`)
  purely to make it mockable; extend those interfaces rather than calling `*hcloud.Client`
  methods directly from new code in this package.

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
