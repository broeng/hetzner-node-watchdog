package health

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// ReadinessChecker reports whether the service backing readiness is currently usable.
// nodecontroller.Controller satisfies this via its node informer's sync state.
type ReadinessChecker interface {
	HasSynced() bool
}

// Controller runs an HTTP server exposing Kubernetes liveness/readiness probe
// endpoints, following the same shape used in
// https://github.com/broeng/livekit-configurator's internal/health package.
type Controller struct {
	logger     logrus.FieldLogger
	ctx        context.Context
	checker    ReadinessChecker
	listenPort int
	isReady    atomic.Bool
}

func New(logger logrus.FieldLogger, ctx context.Context, checker ReadinessChecker, listenPort int) *Controller {
	return &Controller{
		logger:     logger.WithField("component", "health"),
		ctx:        ctx,
		checker:    checker,
		listenPort: listenPort,
	}
}

// Run starts the health HTTP server and blocks until ctx is cancelled.
func (hc *Controller) Run() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", hc.livenessHandler)
	mux.HandleFunc("/readyz", hc.readinessHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", hc.listenPort),
		Handler: mux,
	}

	go func() {
		<-hc.ctx.Done()
		hc.logger.Info("shutting down health HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			hc.logger.Errorf("health HTTP server shutdown error: %s", err)
		}
	}()

	go hc.readinessProbe()

	hc.logger.Infof("health server starting on :%d", hc.listenPort)
	if err := server.ListenAndServe(); err != nil && hc.ctx.Err() == nil {
		hc.logger.Errorf("health server failed: %s", err)
	}

	hc.logger.Info("health HTTP server shut down")
}

// livenessHandler always reports OK: liveness only asks "is the process responding
// at all", not whether it's doing useful work - that's what readiness is for.
func (hc *Controller) livenessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (hc *Controller) readinessHandler(w http.ResponseWriter, r *http.Request) {
	if hc.isReady.Load() {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not Ready: node informer has not synced yet"))
	}
}

func (hc *Controller) readinessProbe() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		hc.isReady.Store(hc.checker.HasSynced())

		select {
		case <-hc.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
