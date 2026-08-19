package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/broeng/hetzner-node-watchdog/internal/config"
	"github.com/broeng/hetzner-node-watchdog/internal/health"
	"github.com/broeng/hetzner-node-watchdog/internal/nodecontroller"
)

const serviceName = "hetzner-node-watchdog"

var version = "unreleased"

func main() {
	logger := logrus.New()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("could not parse options: %s", err)
	}

	if cfg.Version {
		fmt.Println(version)
		os.Exit(0)
	}

	logger.SetLevel(cfg.LogLevel)

	logger.WithFields(logrus.Fields{
		"version":          version,
		"timeout_duration": cfg.TimeoutDuration,
		"grace_period":     cfg.GracePeriod,
	}).Info("starting hetzner node watchdog")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var k8sCfg *rest.Config
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		restCfg, err := rest.InClusterConfig()
		if err != nil {
			logger.Fatalf("could not init in-cluster config: %s", err)
		}
		k8sCfg = restCfg
	} else {
		restCfg, err := clientcmd.BuildConfigFromKubeconfigGetter("", clientcmd.NewDefaultClientConfigLoadingRules().Load)
		if err != nil {
			logger.Fatalf("could not init kubeconfig: %s", err)
		}
		k8sCfg = restCfg
	}

	k8s, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		logger.Fatalf("could not init k8s client: %s", err)
	}

	hcc := hcloud.NewClient(
		hcloud.WithApplication(serviceName, version),
		hcloud.WithToken(cfg.HCloudToken),
		hcloud.WithDebugWriter(logger.WithFields(logrus.Fields{"component": "hcloud"}).WriterLevel(logrus.DebugLevel)),
	)

	nc := nodecontroller.New(logger, k8s, hcc, cfg)
	hc := health.New(logger, ctx, nc, cfg.HealthListenPort)

	var wg sync.WaitGroup
	wg.Go(hc.Run)
	wg.Go(func() {
		nc.Run(ctx)
		stop() // in case nc.Run ever returns before ctx is cancelled, shut health down too
	})
	wg.Wait()

	logger.Info("shutting down")
}
