package config

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stevenroose/gonfig"
	"k8s.io/apimachinery/pkg/labels"
)

// flags mirrors the CLI flags/env vars/config file 1:1 and exists purely as gonfig's
// parse target; nothing outside this package sees it. Load is the only place that
// reads it, validates it, and converts it into a Configuration.
var flags struct {
	LogLevel string `id:"log-level" short:"l" desc:"verbosity level for logs" default:"info"`

	HCloudToken       string `id:"hcloud-token" desc:"API token for Hetzner Cloud access"`
	NodeLabelSelector string `id:"node-label-selector" desc:"label selector used to match nodes to watch" default:""`
	IgnoreCordoned    bool   `id:"ignore-cordoned" desc:"do not restart the server behind a cordoned (unschedulable) node" default:"true"`

	// IgnoreNodeLabelSelector and IgnoreNodeTaintSelector are both "ignore this node
	// entirely" filters (see Configuration.IgnoreNodeLabelSelector/IgnoreNodeTaintSelector):
	// a node matching any entry is neither tracked nor restarted. Each is a list
	// (comma-separated on the CLI/in env vars; quote an entry to embed a literal comma
	// in it), matching if ANY entry matches.
	IgnoreNodeLabelSelector []string `id:"ignore-node-label-selector" desc:"label selector(s); a node matching any of them is never restarted" default:""`
	IgnoreNodeTaintSelector []string `id:"ignore-node-taint-selector" desc:"taint selector(s) in kubectl-taint syntax (key, key=value, key:effect or key=value:effect, with omitted parts acting as wildcards); a node carrying a taint matching any of them is never restarted" default:"node.cloudprovider.kubernetes.io/shutdown"`

	// TimeoutDuration and GracePeriod are both folded into a single "next restart
	// due" timestamp per node (see nodecontroller.nodeState): TimeoutDuration sets
	// that timestamp the first time a node is observed unavailable, GracePeriod
	// resets it after every restart attempt.
	TimeoutDuration *duration `id:"timeout-duration" desc:"how long a node may stay NotReady before its Hetzner server is restarted" default:"10m"`
	GracePeriod     *duration `id:"grace-period" desc:"how long to wait after a restart before restarting again if the node is still unavailable" default:"10m"`

	PollInterval *duration `id:"poll-interval" desc:"how often to check for nodes due a restart" default:"60s" opts:"hidden"`

	HealthListenPort int `id:"port" desc:"listen port for the HTTP health service" default:"8080"`

	Version bool `id:"version" desc:"show version and quit" opts:"hidden"`
}

// Configuration is the fully parsed, ready-to-use configuration for the process; it's
// the only thing the rest of the application depends on. Load is the only place
// gonfig parsing, validation, and conversion into this shape happen.
type Configuration struct {
	// Version, when set, means every other field is left zero: main should just
	// print the binary's version and exit without validating anything else.
	Version bool

	LogLevel logrus.Level

	HCloudToken       string
	NodeLabelSelector string

	IgnoreCordoned          bool
	IgnoreNodeLabelSelector []labels.Selector
	IgnoreNodeTaintSelector []TaintSelector

	TimeoutDuration time.Duration
	GracePeriod     time.Duration
	PollInterval    time.Duration

	HealthListenPort int
}

// Load reads the process configuration from CLI flags, environment variables
// (prefixed NODE_WATCHDOG_) and an optional config file, in increasing order of
// priority, validates it, and returns the parsed Configuration.
func Load() (*Configuration, error) {
	if err := gonfig.Load(&flags, gonfig.Conf{
		EnvPrefix:         "NODE_WATCHDOG_",
		FlagIgnoreUnknown: false,
	}); err != nil {
		return nil, fmt.Errorf("could not parse options: %w", err)
	}

	if flags.Version {
		return &Configuration{Version: true}, nil
	}

	level, err := logrus.ParseLevel(flags.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("could not set log level to %s: %w", flags.LogLevel, err)
	}

	if flags.HCloudToken == "" {
		return nil, fmt.Errorf("hcloud-token (env NODE_WATCHDOG_HCLOUD_TOKEN) is required")
	}

	ignoreNodeLabelSelector, err := parseLabelSelectors(flags.IgnoreNodeLabelSelector)
	if err != nil {
		return nil, fmt.Errorf("--ignore-node-label-selector: %w", err)
	}

	ignoreNodeTaintSelector, err := parseTaintSelectors(flags.IgnoreNodeTaintSelector)
	if err != nil {
		return nil, fmt.Errorf("--ignore-node-taint-selector: %w", err)
	}

	return &Configuration{
		LogLevel: level,

		HCloudToken:       flags.HCloudToken,
		NodeLabelSelector: flags.NodeLabelSelector,

		IgnoreCordoned:          flags.IgnoreCordoned,
		IgnoreNodeLabelSelector: ignoreNodeLabelSelector,
		IgnoreNodeTaintSelector: ignoreNodeTaintSelector,

		TimeoutDuration: time.Duration(*flags.TimeoutDuration),
		GracePeriod:     time.Duration(*flags.GracePeriod),
		PollInterval:    time.Duration(*flags.PollInterval),

		HealthListenPort: flags.HealthListenPort,
	}, nil
}
