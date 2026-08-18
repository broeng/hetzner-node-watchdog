package nodecontroller

import (
	"context"
	"sync"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/broeng/hetzner-node-watchdog/internal/config"
)

// nodeState tracks an unavailable node's restart schedule. nextActionAt is the single
// timestamp driving both TimeoutDuration and GracePeriod: it is set to
// now+TimeoutDuration the first time a node is observed unavailable, and reset to
// now+GracePeriod after every restart attempt. Once "now" passes it while the node is
// still unavailable, the node is due for another restart.
type nodeState struct {
	unavailableSince time.Time
	nextActionAt     time.Time
	restartCount     int
}

type Controller struct {
	logger       logrus.FieldLogger
	k8s          kubernetes.Interface
	hcloudClient hcloudClienter

	state   map[string]*nodeState
	stateMu sync.RWMutex
}

func New(logger logrus.FieldLogger, k8s kubernetes.Interface, hcc *hcloud.Client) *Controller {
	return &Controller{
		logger:       logger.WithField("component", "nodecontroller"),
		k8s:          k8s,
		hcloudClient: hcloudClient{hcc},
		state:        make(map[string]*nodeState),
	}
}

// Run watches node readiness and, in parallel, periodically restarts the Hetzner
// server behind any node that is due (see nodeState). It blocks until ctx is cancelled.
func (c *Controller) Run(ctx context.Context) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		c.k8s,
		time.Duration(*config.Global.PollInterval),
		informers.WithTweakListOptions(func(listOpts *metav1.ListOptions) {
			listOpts.LabelSelector = config.Global.NodeLabelSelector
		}),
	)
	nodeInformer := factory.Core().V1().Nodes().Informer()

	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(newObj interface{}) {
			c.handleNode(newObj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.handleNode(newObj)
		},
		DeleteFunc: func(oldObj interface{}) {
			node, ok := oldObj.(*corev1.Node)
			if !ok {
				tombstone, ok := oldObj.(cache.DeletedFinalStateUnknown)
				if !ok {
					c.logger.Errorf("received unexpected object type: %T", oldObj)
					return
				}
				node, ok = tombstone.Obj.(*corev1.Node)
				if !ok {
					c.logger.Errorf("received unexpected tombstone object type: %T", tombstone.Obj)
					return
				}
			}
			if c.clearNodeState(node.Name) {
				c.logger.WithField("node", node.Name).Info("node removed from cluster; cleared tracked restart state")
			}
		},
	})

	stopper := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(stopper)
	}()

	go c.runRestartLoop(ctx)

	nodeInformer.Run(stopper)
}

func (c *Controller) handleNode(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		c.logger.Errorf("received unexpected object type: %T", obj)
		return
	}

	logger := c.logger.WithField("node", node.Name)

	if nodeIsAvailable(node) {
		if c.clearNodeState(node.Name) {
			logger.Info("node is available again; cleared tracked restart state")
		}
		return
	}

	if restartAt, justTracked := c.trackNodeUnavailable(node.Name); justTracked {
		logger.WithField("restart_due_at", restartAt).Warn("node became unavailable; will restart its Hetzner server if it does not recover in time")
	}
}

func nodeIsAvailable(node *corev1.Node) bool {
	if !node.DeletionTimestamp.IsZero() {
		// node is being removed from the cluster; nothing to restart
		return true
	}

	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}

	// no Ready condition reported yet (e.g. a brand new node); treat as unavailable
	return false
}

// trackNodeUnavailable records the node as unavailable if it isn't already tracked.
// It returns the node's current restart-due timestamp and whether this call started
// tracking it (false if the node was already known to be unavailable).
func (c *Controller) trackNodeUnavailable(name string) (time.Time, bool) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if st, exists := c.state[name]; exists {
		return st.nextActionAt, false
	}

	now := time.Now()
	restartAt := now.Add(time.Duration(*config.Global.TimeoutDuration))
	c.state[name] = &nodeState{
		unavailableSince: now,
		nextActionAt:     restartAt,
	}
	return restartAt, true
}

func (c *Controller) clearNodeState(name string) bool {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if _, exists := c.state[name]; !exists {
		return false
	}
	delete(c.state, name)
	return true
}

func (c *Controller) runRestartLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(*config.Global.PollInterval))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.restartDueNodes(ctx)
		}
	}
}

func (c *Controller) restartDueNodes(ctx context.Context) {
	now := time.Now()

	var due []string
	c.stateMu.RLock()
	for name, st := range c.state {
		if !now.Before(st.nextActionAt) {
			due = append(due, name)
		}
	}
	c.stateMu.RUnlock()

	for _, name := range due {
		c.restartNode(ctx, name)
	}
}

func (c *Controller) restartNode(ctx context.Context, name string) {
	logger := c.logger.WithField("node", name)

	c.stateMu.Lock()
	st, exists := c.state[name]
	if !exists {
		// recovered (or was removed) between restartDueNodes scanning and now
		c.stateMu.Unlock()
		return
	}
	// Reschedule immediately, before the restart attempt runs, so a slow hcloud API
	// call or a failed attempt doesn't get retried on every subsequent poll tick.
	// The node gets a fresh grace period regardless of the outcome below.
	st.nextActionAt = time.Now().Add(time.Duration(*config.Global.GracePeriod))
	st.restartCount++
	attempt := st.restartCount
	graceUntil := st.nextActionAt
	c.stateMu.Unlock()

	logger.WithField("attempt", attempt).Warn("node still unavailable after timeout; restarting its Hetzner server")

	server, _, err := c.hcloudClient.Server().GetByName(ctx, name)
	if err != nil {
		logger.WithError(err).Error("could not look up Hetzner server by node name")
		return
	}
	if server == nil {
		logger.Error("no Hetzner server found matching node name")
		return
	}

	action, _, err := c.hcloudClient.Server().Reset(ctx, server)
	if err != nil {
		logger.WithError(err).Error("could not trigger Hetzner server reset")
		return
	}

	_, errc := c.hcloudClient.Action().WatchProgress(ctx, action)
	if err := <-errc; err != nil {
		logger.WithError(err).Error("Hetzner server reset action failed")
		return
	}

	logger.WithField("grace_period_until", graceUntil).Info("Hetzner server reset triggered; waiting for grace period before checking again")
}
