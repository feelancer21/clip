package main

import (
	"context"
	"fmt"
	"time"

	"github.com/feelancer21/clip"
	"github.com/urfave/cli/v2"
)

var (
	timeoutLightning    = 60 * time.Second
	timeoutNostrPublish = 60 * time.Second
)

type ClipApp struct {
	client *clip.Client
	config *Config
	ctx    *cli.Context
}

func NewApp(c *cli.Context) (*ClipApp, error) {
	cfg, err := loadConfig(c)
	if err != nil {
		return nil, err
	}

	client, err := newClient(c.Context, cfg)
	if err != nil {
		return nil, err
	}

	return &ClipApp{
		client: client,
		config: cfg,
		ctx:    c,
	}, nil
}

func newClient(ctx context.Context, cfg *Config) (*clip.Client, error) {

	keyer, err := loadKeyer(ctx, cfg.KeyStorePath)
	if err != nil {
		return nil, fmt.Errorf("loading keyer: %w", err)
	}

	switch cfg.Lnclient {
	case "lnd":
		ln, err := clip.NewLND(
			cfg.LNDConfig.TLSCertPath,
			cfg.LNDConfig.MacaroonPath,
			cfg.LNDConfig.Host,
			cfg.LNDConfig.Port,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create LND client: %w", err)
		}
		return clip.NewClient(ctx, keyer, ln)

	case "interactive":
		ln := clip.NewLnInteractive(cfg.LnInter.Network, cfg.LnInter.PubKey)
		return clip.NewClient(ctx, keyer, ln)

	default:
		return nil, fmt.Errorf("unsupported lnclient: %s", cfg.Lnclient)
	}
}

func (a *ClipApp) GetInfo() error {
	ctx, cancel := context.WithTimeout(a.ctx.Context, timeoutLightning)
	defer cancel()

	info, err := a.client.GetNodeInfo(ctx)
	if err != nil {
		return fmt.Errorf("getting node info: %w", err)
	}

	return printJSON(info)
}

func (a *ClipApp) ListNodeAnnouncements() error {
	timeout := a.ctx.Duration("timeout")
	ctx, cancel := context.WithTimeout(a.ctx.Context, timeout)
	defer cancel()

	since := a.ctx.Duration("since")
	from := time.Now().Add(-since)

	var pubkeys map[string]struct{}
	if a.ctx.IsSet("pubkey") {
		pubkeys = map[string]struct{}{a.ctx.String("pubkey"): {}}
	}

	showErrors := a.ctx.Bool("show-errors")

	res, err, fetchErrors := clip.GetEventEnvelopes[clip.NodeAnnouncement](a.client,
		ctx, clip.KindNodeAnnouncement, pubkeys, a.config.RelayURLs, from)

	if err != nil {
		return fmt.Errorf("getting event envelopes: %w", err)
	}

	return printSliceJSON(res, fetchErrors, showErrors)
}

func (a *ClipApp) ListNodeInfo() error {
	timeout := a.ctx.Duration("timeout")
	ctx, cancel := context.WithTimeout(a.ctx.Context, timeout)
	defer cancel()

	since := a.ctx.Duration("since")
	from := time.Now().Add(-since)

	var pubkeys map[string]struct{}
	if a.ctx.IsSet("pubkey") {
		pubkeys = map[string]struct{}{a.ctx.String("pubkey"): {}}
	}

	showErrors := a.ctx.Bool("show-errors")

	res, err, fetchErrors := clip.GetEventEnvelopes[clip.NodeInfo](a.client,
		ctx, clip.KindNodeInfo, pubkeys, a.config.RelayURLs, from)

	if err != nil {
		return fmt.Errorf("getting event envelopes: %w", err)
	}

	return printSliceJSON(res, fetchErrors, showErrors)
}

func (a *ClipApp) PublishNodeAnnouncement() error {
	ctx, cancel := context.WithTimeout(a.ctx.Context, timeoutNostrPublish)
	defer cancel()

	data := clip.NodeAnnouncement{}
	res, err := a.client.Publish(ctx, data, clip.KindNodeAnnouncement, a.config.RelayURLs)
	if err != nil {
		return fmt.Errorf("publishing node announcement: %w", err)
	}

	return printPublishResults(res, data)
}

func (a *ClipApp) PublishNodeInfo() error {
	ctx, cancel := context.WithTimeout(a.ctx.Context, timeoutNostrPublish)
	defer cancel()

	data := a.config.NodeInfo
	res, err := a.client.Publish(ctx, data, clip.KindNodeInfo, a.config.RelayURLs)
	if err != nil {
		return fmt.Errorf("publishing node info: %w", err)
	}

	return printPublishResults(res, data)
}

func (a *ClipApp) Close() error {
	return a.client.Close()
}
