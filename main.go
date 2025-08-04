package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/recorder/prometheus"
	"github.com/wafer-bw/jittermon/internal/sampler/exectraceroute"
	"github.com/wafer-bw/jittermon/internal/sampler/grpcp2platency"
	"github.com/wafer-bw/jittermon/internal/sampler/udplatency"
	"golang.org/x/sync/errgroup"
)

type config struct {
	PeerID               string                `envconfig:"PEER_ID" default:""`
	LatencySendAddrs     []string              `envconfig:"LATENCY_SEND_ADDRS" default:"8.8.8.8:53"`
	LatencyInterval      time.Duration         `envconfig:"LATENCY_INTERVAL" default:"0.25s"`
	P2PLatencyListenAddr string                `envconfig:"P2P_LATENCY_LISTEN_ADDR" default:""`
	P2PLatencySendAddrs  []string              `envconfig:"P2P_LATENCY_SEND_ADDRS" default:""`
	P2PLatencyInterval   time.Duration         `envconfig:"P2P_LATENCY_INTERVAL" default:"1s"`
	TraceSendAddrs       []string              `envconfig:"TRACE_SEND_ADDRS" default:""`
	TraceInterval        time.Duration         `envconfig:"TRACE_INTERVAL" default:"1s"`
	TraceMaxHops         int                   `envconfig:"TRACE_MAX_HOPS" default:"12"`
	Metrics              []recorder.SampleType `envconfig:"METRICS" default:"rtt,hop_rtt,downstream_jitter,upstream_jitter,sent_packets,lost_packets,rtt_jitter"`
	MetricsAddr          string                `envconfig:"METRICS_ADDR" default:""`
	LogLevel             slog.Level            `envconfig:"LOG_LEVEL" default:"INFO"`
}

func main() {
	ctx := context.Background()
	conf := config{}
	envconfig.MustProcess("JITTERMON", &conf)

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: conf.LogLevel}))

	if err := run(ctx, log, conf); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger, conf config) error {
	eg, ctx := errgroup.WithContext(ctx)

	recorders := []recorder.ChainLink{recorder.MetricFilter(conf.Metrics...)}
	if conf.MetricsAddr != "" {
		prometheus, err := prometheus.New(conf.MetricsAddr,
			prometheus.WithLog(log),
			prometheus.WithID(conf.PeerID),
		)
		if err != nil {
			return err
		}
		eg.Go(func() error { return prometheus.Run(ctx) })
		recorders = append(recorders, prometheus.DefaultRecorders()...)
	}
	chain := recorder.Chain(recorders...)

	for _, addr := range conf.LatencySendAddrs {
		client, err := udplatency.New(addr, chain,
			udplatency.WithID(conf.PeerID),
			udplatency.WithInterval(conf.LatencyInterval),
			udplatency.WithLog(log),
		)
		if err != nil {
			return err
		}
		eg.Go(func() error { return client.Run(ctx) })
	}

	if conf.P2PLatencyListenAddr != "" {
		server, err := grpcp2platency.NewServer(conf.P2PLatencyListenAddr, chain,
			grpcp2platency.WithServerID(conf.PeerID),
			grpcp2platency.WithServerLog(log),
		)
		if err != nil {
			return err
		}
		eg.Go(func() error { return server.Run(ctx) })
	}

	for _, addr := range conf.P2PLatencySendAddrs {
		client, err := grpcp2platency.NewClient(addr, chain,
			grpcp2platency.WithClientID(conf.PeerID),
			grpcp2platency.WithClientInterval(conf.P2PLatencyInterval),
			grpcp2platency.WithClientLog(log),
		)
		if err != nil {
			return err
		}
		eg.Go(func() error { return client.Run(ctx) })
	}

	for _, addr := range conf.TraceSendAddrs {
		client, err := exectraceroute.NewTraceRoute(addr, chain,
			exectraceroute.WithID(conf.PeerID),
			exectraceroute.WithInterval(conf.TraceInterval),
			exectraceroute.WithMaxHops(conf.TraceMaxHops),
			exectraceroute.WithLog(log),
		)
		if err != nil {
			return err
		}
		eg.Go(func() error { return client.Run(ctx) })
	}

	return eg.Wait()
}
