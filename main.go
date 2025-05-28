package main

import (
	"context"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/recorder/logger"
	"github.com/wafer-bw/jittermon/internal/recorder/prometheus"
	"github.com/wafer-bw/jittermon/internal/recorder/store"
	"github.com/wafer-bw/jittermon/internal/sampler/latency"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platency"
	"github.com/wafer-bw/jittermon/internal/sampler/traceroute"
)

const shutdownTimeout time.Duration = 1 * time.Second

type config struct {
	PeerID                string                `envconfig:"PEER_ID" default:""`
	LatencySendAddrs      []string              `envconfig:"LATENCY_SEND_ADDRS" default:"8.8.8.8:53"`
	LatencyInterval       time.Duration         `envconfig:"LATENCY_INTERVAL" default:"0.25s"`
	P2PLatencyListenAddr  string                `envconfig:"P2P_LATENCY_LISTEN_ADDR" default:""`
	P2PLatencySendAddrs   []string              `envconfig:"P2P_LATENCY_SEND_ADDRS" default:""`
	P2PLatencyInterval    time.Duration         `envconfig:"P2P_LATENCY_INTERVAL" default:"1s"`
	TraceSendAddrs        []string              `envconfig:"TRACE_SEND_ADDRS" default:""`
	TraceInterval         time.Duration         `envconfig:"TRACE_INTERVAL" default:"1s"`
	TraceMaxHops          int                   `envconfig:"TRACE_MAX_HOPS" default:"12"`
	Metrics               []recorder.SampleType `envconfig:"METRICS" default:"rtt,hop_rtt,downstream_jitter,upstream_jitter,sent_packets,lost_packets,rtt_jitter"`
	MetricsAddr           string                `envconfig:"METRICS_ADDR" default:""`
	LogLevel              slog.Level            `envconfig:"LOG_LEVEL" default:"INFO"`
	UseLogRecorder        bool                  `envconfig:"USE_LOG_RECORDER" default:"false"`
	UseLocalStoreRecorder bool                  `envconfig:"USE_LOCAL_STORE_RECORDER" default:"true"`
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
	group := graceful.Group{}
	recorders := []recorder.ChainLink{recorder.MetricFilter(conf.Metrics...)}
	exitSignals := []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	localStore, err := store.New(store.WithLogger(log))
	if err != nil {
		return err
	}
	group = append(group, localStore)

	if conf.UseLogRecorder {
		recorders = append(recorders, logger.Recorder(log))
	}

	if conf.UseLocalStoreRecorder {
		recorders = append(recorders, localStore.Recorder)
	}

	if conf.MetricsAddr != "" {
		prometheus, err := prometheus.New(conf.MetricsAddr, log)
		if err != nil {
			return err
		}
		group = append(group, prometheus)
		recorders = append(recorders, prometheus.DefaultRecorders()...)
	}

	chain := recorder.Chain(recorders...)

	for _, addr := range conf.LatencySendAddrs {
		client := &latency.Client{
			ID:             conf.PeerID,
			Address:        addr,
			Interval:       conf.LatencyInterval,
			Recorder:       chain,
			RequestBuffers: &jitter.Buffer{},
			Log:            log,
			StopCh:         make(chan struct{}),
			StoppedCh:      make(chan struct{}),
		}
		group = append(group, client)
	}

	peer, err := p2platency.NewPeer(
		p2platency.WithID(conf.PeerID),
		p2platency.WithListenAddress(conf.P2PLatencyListenAddr),
		p2platency.WithSendAddresses(conf.P2PLatencySendAddrs...),
		p2platency.WithInterval(conf.P2PLatencyInterval),
		p2platency.WithRecorder(chain),
		p2platency.WithLog(log),
	)
	if err != nil {
		return err
	}
	group = append(group, peer)

	for _, addr := range conf.TraceSendAddrs {
		traceRouteSampler, err := traceroute.NewTraceRoute(
			traceroute.WithID(conf.PeerID),
			traceroute.WithAddress(addr),
			traceroute.WithInterval(conf.TraceInterval),
			traceroute.WithMaxHops(conf.TraceMaxHops),
			traceroute.WithRecorder(chain),
			traceroute.WithLog(log),
		)
		if err != nil {
			return err
		}
		group = append(group, traceRouteSampler)
	}

	if err := group.Run(ctx,
		graceful.WithStopTimeout(shutdownTimeout),
		graceful.WithStopSignals(exitSignals...),
	); err != nil {
		return err
	}

	return nil
}
