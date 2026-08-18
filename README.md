# hetzner-node-watchdog

A Kubernetes controller that watches node `Ready` status and, if a node stays
unavailable for too long, restarts the matching Hetzner Cloud server via the
[hcloud-go](https://github.com/hetznercloud/hcloud-go) library.

Nodes are matched to Hetzner Cloud servers **by name**, so your Kubernetes
node names must be identical to the corresponding Hetzner Cloud server names
(this is the default on most Hetzner-based Kubernetes setups, e.g. clusters
provisioned with Hetzner's cloud-controller-manager).

## How it works

For each node, the controller tracks a single `restart due at` timestamp:

- When a node's `Ready` condition goes false (or is missing), and it wasn't
  already being tracked, that timestamp is set to `now + TIMEOUT_DURATION`.
- Once "now" passes that timestamp and the node is *still* unavailable, the
  controller looks up the Hetzner Cloud server with the same name and issues
  a hardware reset (`Server.Reset`, closer to a physical reset button than a
  graceful reboot — appropriate for a node that isn't responding at all).
  The timestamp is then pushed forward to `now + GRACE_PERIOD` so the node
  gets a chance to come back up before another restart is attempted.
- As soon as the node reports `Ready` again, tracking is cleared.

This single-timestamp approach means `TIMEOUT_DURATION` and `GRACE_PERIOD`
are handled by the same code path: the first one just seeds it, the second
one keeps resetting it after every restart attempt.

Only one replica should run at a time: restart state is kept in memory with
no leader election, so a second replica could race and trigger duplicate
restarts.

## Configuration

Configuration follows the same [gonfig](https://github.com/stevenroose/gonfig)-based
approach as [hcloud-ip-floater](https://github.com/broeng/hcloud-ip-floater):
values can come from CLI flags, environment variables (prefix `NODE_WATCHDOG_`),
or a config file, in increasing order of priority.

| Variable                          | Flag                    | Purpose                                                                 | Default |
|------------------------------------|-------------------------|--------------------------------------------------------------------------|---------|
| `NODE_WATCHDOG_HCLOUD_TOKEN`        | `--hcloud-token`        | Hetzner Cloud API token (required)                                       | —       |
| `NODE_WATCHDOG_NODE_LABEL_SELECTOR` | `--node-label-selector` | Label selector to restrict which nodes are watched                       | `""` (all nodes) |
| `NODE_WATCHDOG_TIMEOUT_DURATION`    | `--timeout-duration`    | How long a node may stay `NotReady` before its server is restarted       | `10m`   |
| `NODE_WATCHDOG_GRACE_PERIOD`        | `--grace-period`        | How long to wait after a restart before restarting again if still down   | `10m`   |
| `NODE_WATCHDOG_LOG_LEVEL`           | `--log-level` / `-l`    | Log verbosity (`debug`/`info`/`warn`/`error`)                            | `info`  |

Durations use Go's [`time.ParseDuration`](https://pkg.go.dev/time#ParseDuration)
format (e.g. `90s`, `5m`, `1h30m`).

Run `./hetzner-node-watchdog --help` for the full, current list of flags.

## Running

Locally, against whatever cluster your kubeconfig points at:

```sh
export NODE_WATCHDOG_HCLOUD_TOKEN=...
go run . --log-level=debug
```

In-cluster, it auto-detects `KUBERNETES_SERVICE_HOST` and uses the pod's
service account instead of a kubeconfig.

## Deploying with Helm

`deploy/` is a Helm chart. Every application CLI flag is exposed as a
`values.yaml` setting under `config.*` and rendered as a `--flag=value`
argument on the container; see `deploy/values.yaml` for the full list and
defaults.

```sh
helm install hetzner-node-watchdog ./deploy \
  --create-namespace -n hetzner-node-watchdog \
  --set hcloud.token=$NODE_WATCHDOG_HCLOUD_TOKEN
```

The Hetzner token is always injected via a Secret and referenced through
`$(HCLOUD_TOKEN)` in the container args, never templated into the pod spec
directly. Set `hcloud.existingSecret` (+ `hcloud.existingSecretKey`, default
`hcloud-token`) instead of `hcloud.token` to reference a Secret you manage
yourself. The chart also creates the ServiceAccount and the minimal
ClusterRole it needs (`get`/`watch`/`list` on `nodes`).

## Building

```sh
go mod tidy   # resolves go.sum; not committed here since this scaffold was
              # generated without a local Go toolchain to run it
go build .
```

Or via Docker: `docker build -t hetzner-node-watchdog .`
