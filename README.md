# hetzner-node-watchdog

A Kubernetes controller that watches node `Ready` status and, if a node stays
unavailable too long, restarts the matching Hetzner Cloud server with a hardware
reset (via [hcloud-go](https://github.com/hetznercloud/hcloud-go)).

Nodes are matched to Hetzner Cloud servers **by name**, so your Kubernetes node
names must match the corresponding server names (the default on most
Hetzner-based Kubernetes setups).

Restart state is kept in memory with no leader election, so **only run one
replica** — see "Redundancy without leader election" below for a supported way
to add a backup instance instead of scaling up.

This utility has been produced using Claude.

## Configuration

Settings can come from CLI flags, environment variables (prefix `NODE_WATCHDOG_`),
or a config file, in increasing order of priority. Durations use Go's
[`time.ParseDuration`](https://pkg.go.dev/time#ParseDuration) format (e.g. `90s`,
`5m`, `1h30m`). Run `./hetzner-node-watchdog --help` for the full, current list.

| Variable | Flag | Purpose | Default |
|---|---|---|---|
| `NODE_WATCHDOG_HCLOUD_TOKEN` | `--hcloud-token` | Hetzner Cloud API token (required) | — |
| `NODE_WATCHDOG_NODE_LABEL_SELECTOR` | `--node-label-selector` | Restrict which nodes are watched | `""` (all) |
| `NODE_WATCHDOG_IGNORE_CORDONED` | `--ignore-cordoned` | Skip cordoned nodes | `true` |
| `NODE_WATCHDOG_IGNORE_NODE_LABEL_SELECTOR` | `--ignore-node-label-selector` | Label selector(s) (comma-separated, OR'd); matching nodes are never restarted | `""` (none) |
| `NODE_WATCHDOG_IGNORE_NODE_TAINT_SELECTOR` | `--ignore-node-taint-selector` | Taint selector(s) in `kubectl taint` syntax (`key`, `key=value`, `key:effect`, `key=value:effect`); matching nodes are never restarted | `node.cloudprovider.kubernetes.io/shutdown` |
| `NODE_WATCHDOG_TIMEOUT_DURATION` | `--timeout-duration` | How long a node may stay `NotReady` before its server is restarted | `10m` |
| `NODE_WATCHDOG_GRACE_PERIOD` | `--grace-period` | How long to wait after a restart before restarting again if still down | `10m` |
| `NODE_WATCHDOG_LOG_LEVEL` | `--log-level` / `-l` | Log verbosity (`debug`/`info`/`warn`/`error`) | `info` |
| `NODE_WATCHDOG_PORT` | `--port` | Listen port for the HTTP health service | `8080` |

## Running

Locally, against whatever cluster your kubeconfig points at:

```sh
export NODE_WATCHDOG_HCLOUD_TOKEN=...
go run . --log-level=debug
```

In-cluster, it auto-detects `KUBERNETES_SERVICE_HOST` and uses the pod's
service account instead of a kubeconfig.

### Health checks

`/healthz` (liveness, always `200`) and `/readyz` (readiness, `200` once the
node informer has synced) are wired into the Helm chart's probes automatically.

## Deploying with Helm

`deploy/` is a Helm chart. Every CLI flag is exposed as a `values.yaml` setting
under `config.*` and rendered as a `--flag=value` argument on the container; see
`deploy/values.yaml` for the full list and defaults.

```sh
helm install hetzner-node-watchdog ./deploy \
  --create-namespace -n hetzner-node-watchdog \
  --set hcloud.token=$NODE_WATCHDOG_HCLOUD_TOKEN
```

The Hetzner token is always injected via a Secret, never passed as a plaintext
arg. Set `hcloud.existingSecret` (+ `hcloud.existingSecretKey`, default
`hcloud-token`) instead of `hcloud.token` to reference a Secret you manage
yourself. The chart also creates the ServiceAccount and the minimal ClusterRole
it needs (`get`/`watch`/`list` on `nodes`).

### Redundancy without leader election

Set `secondary.enabled=true` to run a second "secondary" Deployment alongside
the primary, with pod anti-affinity so the two never land on the same node.
The secondary uses its own, much longer `secondary.config.timeoutDuration`/
`gracePeriod` (default `60m`/`60m`) — any field left unset there falls back to
the primary's value. Both instances watch and act independently; the
secondary's long timeout is what stops it from racing the primary under normal
conditions, at the cost of a real (if rare) window where an unavailable node
goes unrestarted until the secondary's own timeout elapses, should the primary
itself go down.
