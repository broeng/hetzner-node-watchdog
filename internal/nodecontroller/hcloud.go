package nodecontroller

import (
	"context"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

// hcloudClienter wraps a thin interface around the hcloud.Client to make it mockable in tests.
type hcloudClienter interface {
	Server() hcloudServerer
	Action() hcloudActioner
}

type hcloudClient struct {
	*hcloud.Client
}

func (hcc hcloudClient) Server() hcloudServerer {
	return &hcc.Client.Server
}

func (hcc hcloudClient) Action() hcloudActioner {
	return &hcc.Client.Action
}

type hcloudServerer interface {
	GetByName(context.Context, string) (*hcloud.Server, *hcloud.Response, error)
	// Reset performs a hardware reset (similar to pressing a physical reset button),
	// which is more likely than a graceful Reboot to recover a server whose OS is
	// already unresponsive - the situation this controller reacts to.
	Reset(context.Context, *hcloud.Server) (*hcloud.Action, *hcloud.Response, error)
}

type hcloudActioner interface {
	WatchProgress(context.Context, *hcloud.Action) (<-chan int, <-chan error)
}
