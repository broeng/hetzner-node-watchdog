package config

// Global holds the fully loaded configuration for the process. It is populated once
// in main() via gonfig.Load, which reads (in increasing priority) struct defaults,
// a config file, environment variables (prefixed with NODE_WATCHDOG_) and CLI flags.
var Global struct {
	LogLevel string `id:"log-level" short:"l" desc:"verbosity level for logs" default:"info"`

	HCloudToken       string `id:"hcloud-token" desc:"API token for Hetzner Cloud access"`
	NodeLabelSelector string `id:"node-label-selector" desc:"label selector used to match nodes to watch" default:""`

	// TimeoutDuration and GracePeriod are both folded into a single "next restart
	// due" timestamp per node (see nodecontroller.nodeState): TimeoutDuration sets
	// that timestamp the first time a node is observed unavailable, GracePeriod
	// resets it after every restart attempt.
	TimeoutDuration *Duration `id:"timeout-duration" desc:"how long a node may stay NotReady before its Hetzner server is restarted" default:"10m"`
	GracePeriod     *Duration `id:"grace-period" desc:"how long to wait after a restart before restarting again if the node is still unavailable" default:"10m"`

	PollInterval *Duration `id:"poll-interval" desc:"how often to check for nodes due a restart" default:"60s" opts:"hidden"`

	HealthListenPort int `id:"port" desc:"listen port for the HTTP health service" default:"8080"`

	Version bool `id:"version" desc:"show version and quit" opts:"hidden"`
}
