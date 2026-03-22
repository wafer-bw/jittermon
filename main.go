package main

import (
	"cmp"
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/wafer-bw/jittermon/internal/littleid"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/recorder/prometheus"
	"github.com/wafer-bw/jittermon/internal/sampler/grpcp2platency"
	"github.com/wafer-bw/jittermon/internal/sampler/udplatency"
	"golang.org/x/sync/errgroup"
)

type config struct {
	ID                   string                `envconfig:"PEER_ID" default:""`
	LatencySendAddrs     []string              `envconfig:"PING_ADDRS" default:"8.8.8.8:53"`
	LatencyInterval      time.Duration         `envconfig:"PING_INTERVAL" default:"0.25s"`
	P2PLatencyListenAddr string                `envconfig:"JITTER_LISTEN_ADDR" default:""`
	P2PLatencySendAddrs  []string              `envconfig:"JITTER_SEND_ADDRS" default:""`
	P2PLatencyInterval   time.Duration         `envconfig:"JITTER_INTERVAL" default:"1s"`
	Metrics              []recorder.SampleType `envconfig:"METRICS" default:"rtt,downstream_jitter,upstream_jitter,rtt_jitter,sent_packets,lost_packets"`
	MetricsAddr          string                `envconfig:"METRICS_ADDR" default:""`
	LogLevel             slog.Level            `envconfig:"LOG_LEVEL" default:"INFO"`
}

func main() {
	ctx := context.Background()

	cfg := config{}
	envconfig.MustProcess("JITTERMON", &cfg)
	if cfg.ID == "" {
		cfg.ID = cmp.Or(os.Getenv("HOSTNAME"), littleid.New())
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))

	if err := run(ctx, log, cfg); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger, cfg config) error {
	eg, ctx := errgroup.WithContext(ctx)

	recorders := []recorder.ChainLink{recorder.MetricFilter(cfg.Metrics...)}
	if cfg.MetricsAddr != "" {
		prometheus, err := prometheus.New(cfg.MetricsAddr,
			prometheus.WithLog(log),
			prometheus.WithID(cfg.ID),
		)
		if err != nil {
			return err
		}
		eg.Go(func() error { return prometheus.Run(ctx) })
		recorders = append(recorders, prometheus.DefaultRecorders()...)
	}
	chain := recorder.Chain(recorders...)

	for _, addr := range cfg.LatencySendAddrs {
		client, err := udplatency.New(cfg.ID, addr, chain,
			udplatency.WithInterval(cfg.LatencyInterval),
			udplatency.WithLog(log),
		)
		if err != nil {
			return err
		}
		eg.Go(func() error { return client.Run(ctx) })
	}

	if cfg.P2PLatencyListenAddr != "" {
		server, err := grpcp2platency.NewServer(cfg.P2PLatencyListenAddr, chain,
			grpcp2platency.WithServerID(cfg.ID),
			grpcp2platency.WithServerLog(log),
		)
		if err != nil {
			return err
		}
		eg.Go(func() error { return server.Run(ctx) })
	}

	for _, addr := range cfg.P2PLatencySendAddrs {
		client, err := grpcp2platency.NewClient(addr, chain,
			grpcp2platency.WithClientID(cfg.ID),
			grpcp2platency.WithClientInterval(cfg.P2PLatencyInterval),
			grpcp2platency.WithClientLog(log),
		)
		if err != nil {
			return err
		}
		eg.Go(func() error { return client.Run(ctx) })
	}

	return eg.Wait()
}
