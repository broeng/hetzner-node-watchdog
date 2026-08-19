package config

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// withArgs replaces os.Args for the duration of the test and restores it afterwards.
// Load reads os.Args directly (via gonfig), so tests must fully control it rather than
// relying on `go test`'s own flags (e.g. -test.v) not tripping gonfig's flag parser,
// which errors on any argument it doesn't recognize as one of our own flags.
func withArgs(t *testing.T, args ...string) {
	t.Helper()
	orig := os.Args
	os.Args = append([]string{"hetzner-node-watchdog"}, args...)
	t.Cleanup(func() { os.Args = orig })
}

func TestLoadRequiresHCloudToken(t *testing.T) {
	withArgs(t)
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when hcloud-token is missing, got none")
	}
	if !strings.Contains(err.Error(), "hcloud-token") {
		t.Errorf("error %q does not mention hcloud-token", err.Error())
	}
}

func TestLoadDefaults(t *testing.T) {
	withArgs(t, "--hcloud-token=secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Version {
		t.Error("Version should be false by default")
	}
	if cfg.LogLevel != logrus.InfoLevel {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, logrus.InfoLevel)
	}
	if cfg.HCloudToken != "secret" {
		t.Errorf("HCloudToken = %q, want %q", cfg.HCloudToken, "secret")
	}
	if cfg.NodeLabelSelector != "" {
		t.Errorf("NodeLabelSelector = %q, want empty", cfg.NodeLabelSelector)
	}
	if !cfg.IgnoreCordoned {
		t.Error("IgnoreCordoned should default to true")
	}
	if len(cfg.IgnoreNodeLabelSelector) != 0 {
		t.Errorf("IgnoreNodeLabelSelector = %v, want empty", cfg.IgnoreNodeLabelSelector)
	}
	if len(cfg.IgnoreNodeTaintSelector) != 1 {
		t.Fatalf("IgnoreNodeTaintSelector = %v, want exactly the shutdown taint", cfg.IgnoreNodeTaintSelector)
	}
	wantShutdown := TaintSelector{Key: "node.cloudprovider.kubernetes.io/shutdown"}
	if cfg.IgnoreNodeTaintSelector[0] != wantShutdown {
		t.Errorf("IgnoreNodeTaintSelector[0] = %+v, want %+v", cfg.IgnoreNodeTaintSelector[0], wantShutdown)
	}
	if cfg.TimeoutDuration != 10*time.Minute {
		t.Errorf("TimeoutDuration = %v, want 10m", cfg.TimeoutDuration)
	}
	if cfg.GracePeriod != 10*time.Minute {
		t.Errorf("GracePeriod = %v, want 10m", cfg.GracePeriod)
	}
	if cfg.PollInterval != 60*time.Second {
		t.Errorf("PollInterval = %v, want 60s", cfg.PollInterval)
	}
	if cfg.HealthListenPort != 8080 {
		t.Errorf("HealthListenPort = %d, want 8080", cfg.HealthListenPort)
	}
}

func TestLoadVersionShortCircuitsOtherValidation(t *testing.T) {
	// No hcloud-token and a bogus log level would both normally fail validation;
	// --version must skip all of that and return immediately.
	withArgs(t, "--version", "--log-level=not-a-level")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Version {
		t.Error("expected Version to be true")
	}
}

func TestLoadInvalidLogLevel(t *testing.T) {
	withArgs(t, "--hcloud-token=secret", "--log-level=not-a-level")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid log level, got none")
	}
	if !strings.Contains(err.Error(), "log level") {
		t.Errorf("error %q does not mention log level", err.Error())
	}
}

func TestLoadLogLevels(t *testing.T) {
	tests := []struct {
		in   string
		want logrus.Level
	}{
		{"debug", logrus.DebugLevel},
		{"info", logrus.InfoLevel},
		{"warn", logrus.WarnLevel},
		{"error", logrus.ErrorLevel},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			withArgs(t, "--hcloud-token=secret", "--log-level="+tt.in)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.LogLevel != tt.want {
				t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, tt.want)
			}
		})
	}
}

func TestLoadNodeLabelSelectorPassesThroughRaw(t *testing.T) {
	// Unlike the ignore-node-* selectors, NodeLabelSelector is handed to the
	// Kubernetes API list options as-is and must not be parsed/altered locally.
	withArgs(t, "--hcloud-token=secret", "--node-label-selector=role=worker,zone in (eu,us)")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "role=worker,zone in (eu,us)"
	if cfg.NodeLabelSelector != want {
		t.Errorf("NodeLabelSelector = %q, want %q", cfg.NodeLabelSelector, want)
	}
}

func TestLoadIgnoreCordonedCanBeDisabled(t *testing.T) {
	withArgs(t, "--hcloud-token=secret", "--ignore-cordoned=false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.IgnoreCordoned {
		t.Error("expected IgnoreCordoned to be false")
	}
}

func TestLoadDurations(t *testing.T) {
	withArgs(t, "--hcloud-token=secret", "--timeout-duration=5m", "--grace-period=90s", "--poll-interval=15s")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TimeoutDuration != 5*time.Minute {
		t.Errorf("TimeoutDuration = %v, want 5m", cfg.TimeoutDuration)
	}
	if cfg.GracePeriod != 90*time.Second {
		t.Errorf("GracePeriod = %v, want 90s", cfg.GracePeriod)
	}
	if cfg.PollInterval != 15*time.Second {
		t.Errorf("PollInterval = %v, want 15s", cfg.PollInterval)
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	withArgs(t, "--hcloud-token=secret", "--timeout-duration=not-a-duration")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid duration, got none")
	}
}

func TestLoadIgnoreNodeLabelSelectorSingleValue(t *testing.T) {
	withArgs(t, "--hcloud-token=secret", "--ignore-node-label-selector=role=spot")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.IgnoreNodeLabelSelector) != 1 {
		t.Fatalf("got %d selectors, want 1", len(cfg.IgnoreNodeLabelSelector))
	}
	if !cfg.IgnoreNodeLabelSelector[0].Matches(labels.Set{"role": "spot"}) {
		t.Error("expected selector to match role=spot")
	}
	if cfg.IgnoreNodeLabelSelector[0].Matches(labels.Set{"role": "ondemand"}) {
		t.Error("expected selector not to match role=ondemand")
	}
}

func TestLoadIgnoreNodeLabelSelectorMultipleValuesAreOred(t *testing.T) {
	withArgs(t, "--hcloud-token=secret", "--ignore-node-label-selector=role=spot,env=staging")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.IgnoreNodeLabelSelector) != 2 {
		t.Fatalf("got %d selectors, want 2 (comma splits into separate OR'd entries)", len(cfg.IgnoreNodeLabelSelector))
	}

	matchesAny := func(set labels.Set) bool {
		for _, sel := range cfg.IgnoreNodeLabelSelector {
			if sel.Matches(set) {
				return true
			}
		}
		return false
	}

	if !matchesAny(labels.Set{"role": "spot"}) {
		t.Error("expected a node with only role=spot to match")
	}
	if !matchesAny(labels.Set{"env": "staging"}) {
		t.Error("expected a node with only env=staging to match")
	}
	if matchesAny(labels.Set{"role": "ondemand", "env": "prod"}) {
		t.Error("expected a node matching neither to not match")
	}
}

func TestLoadIgnoreNodeLabelSelectorQuotedCommaStaysOneSelector(t *testing.T) {
	// gonfig's slice parsing is CSV-based: an unquoted comma always splits into two
	// list entries, so a single selector that itself needs a comma (e.g. a set-based
	// "in (a,b)" expression) must be double-quoted to survive as one entry.
	withArgs(t, "--hcloud-token=secret", `--ignore-node-label-selector="role in (spot,ondemand)"`)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.IgnoreNodeLabelSelector) != 1 {
		t.Fatalf("got %d selectors, want 1 (quoting should keep it as a single entry)", len(cfg.IgnoreNodeLabelSelector))
	}
	if !cfg.IgnoreNodeLabelSelector[0].Matches(labels.Set{"role": "spot"}) {
		t.Error("expected quoted set-based selector to match role=spot")
	}
	if !cfg.IgnoreNodeLabelSelector[0].Matches(labels.Set{"role": "ondemand"}) {
		t.Error("expected quoted set-based selector to match role=ondemand")
	}
}

func TestLoadInvalidIgnoreNodeLabelSelector(t *testing.T) {
	withArgs(t, "--hcloud-token=secret", "--ignore-node-label-selector=???not a selector???")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid label selector, got none")
	}
	if !strings.Contains(err.Error(), "ignore-node-label-selector") {
		t.Errorf("error %q does not mention --ignore-node-label-selector", err.Error())
	}
}

func TestLoadIgnoreNodeTaintSelectorFormats(t *testing.T) {
	tests := []struct {
		name string
		flag string
		want TaintSelector
	}{
		{"key only", "foo", TaintSelector{Key: "foo"}},
		{"key=value", "foo=bar", TaintSelector{Key: "foo", Value: "bar", HasValue: true}},
		{"key:effect", "foo:NoSchedule", TaintSelector{Key: "foo", Effect: corev1.TaintEffectNoSchedule, HasEffect: true}},
		{"key=value:effect", "foo=bar:NoExecute", TaintSelector{Key: "foo", Value: "bar", HasValue: true, Effect: corev1.TaintEffectNoExecute, HasEffect: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withArgs(t, "--hcloud-token=secret", "--ignore-node-taint-selector="+tt.flag)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(cfg.IgnoreNodeTaintSelector) != 1 {
				t.Fatalf("got %d selectors, want 1", len(cfg.IgnoreNodeTaintSelector))
			}
			if cfg.IgnoreNodeTaintSelector[0] != tt.want {
				t.Errorf("got %+v, want %+v", cfg.IgnoreNodeTaintSelector[0], tt.want)
			}
		})
	}
}

func TestLoadIgnoreNodeTaintSelectorMultipleValues(t *testing.T) {
	withArgs(t, "--hcloud-token=secret", "--ignore-node-taint-selector=foo,bar:NoExecute")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.IgnoreNodeTaintSelector) != 2 {
		t.Fatalf("got %d selectors, want 2", len(cfg.IgnoreNodeTaintSelector))
	}
}

func TestLoadInvalidIgnoreNodeTaintSelector(t *testing.T) {
	withArgs(t, "--hcloud-token=secret", "--ignore-node-taint-selector==bad")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid taint selector, got none")
	}
	if !strings.Contains(err.Error(), "ignore-node-taint-selector") {
		t.Errorf("error %q does not mention --ignore-node-taint-selector", err.Error())
	}
}

func TestLoadFromEnvVars(t *testing.T) {
	withArgs(t)
	t.Setenv("NODE_WATCHDOG_HCLOUD_TOKEN", "env-secret")
	t.Setenv("NODE_WATCHDOG_LOG_LEVEL", "debug")
	t.Setenv("NODE_WATCHDOG_TIMEOUT_DURATION", "3m")
	t.Setenv("NODE_WATCHDOG_IGNORE_CORDONED", "false")
	t.Setenv("NODE_WATCHDOG_IGNORE_NODE_TAINT_SELECTOR", "foo=bar")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HCloudToken != "env-secret" {
		t.Errorf("HCloudToken = %q, want %q", cfg.HCloudToken, "env-secret")
	}
	if cfg.LogLevel != logrus.DebugLevel {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
	if cfg.TimeoutDuration != 3*time.Minute {
		t.Errorf("TimeoutDuration = %v, want 3m", cfg.TimeoutDuration)
	}
	if cfg.IgnoreCordoned {
		t.Error("expected IgnoreCordoned to be false")
	}
	want := TaintSelector{Key: "foo", Value: "bar", HasValue: true}
	if len(cfg.IgnoreNodeTaintSelector) != 1 || cfg.IgnoreNodeTaintSelector[0] != want {
		t.Errorf("IgnoreNodeTaintSelector = %+v, want [%+v]", cfg.IgnoreNodeTaintSelector, want)
	}
}

func TestLoadFlagsTakePriorityOverEnvVars(t *testing.T) {
	withArgs(t, "--hcloud-token=secret", "--timeout-duration=2m")
	t.Setenv("NODE_WATCHDOG_TIMEOUT_DURATION", "1m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TimeoutDuration != 2*time.Minute {
		t.Errorf("TimeoutDuration = %v, want 2m (flag should win over env var)", cfg.TimeoutDuration)
	}
}

func TestLoadEnvVarsTakePriorityOverDefaults(t *testing.T) {
	withArgs(t, "--hcloud-token=secret")
	t.Setenv("NODE_WATCHDOG_GRACE_PERIOD", "42m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GracePeriod != 42*time.Minute {
		t.Errorf("GracePeriod = %v, want 42m (env var should win over default)", cfg.GracePeriod)
	}
}

func TestLoadHealthListenPort(t *testing.T) {
	withArgs(t, "--hcloud-token=secret", "--port=9090")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HealthListenPort != 9090 {
		t.Errorf("HealthListenPort = %d, want 9090", cfg.HealthListenPort)
	}
}

func TestLoadStateIsIndependentBetweenCalls(t *testing.T) {
	// Regression guard: Load must not leak state between calls (e.g. via a shared
	// package-level parse target), since the process only calls it once but tests
	// call it many times back-to-back.
	withArgs(t, "--hcloud-token=first", "--ignore-node-label-selector=role=spot")
	first, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(first.IgnoreNodeLabelSelector) != 1 {
		t.Fatalf("first load: got %d selectors, want 1", len(first.IgnoreNodeLabelSelector))
	}

	withArgs(t, "--hcloud-token=second")
	second, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(second.IgnoreNodeLabelSelector) != 0 {
		t.Errorf("second load: got %d selectors, want 0 (leaked from previous call)", len(second.IgnoreNodeLabelSelector))
	}
	if second.HCloudToken != "second" {
		t.Errorf("second load: HCloudToken = %q, want %q", second.HCloudToken, "second")
	}
}
