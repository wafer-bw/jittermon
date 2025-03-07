package main

import (
	"context"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platency"
	"github.com/wafer-bw/jittermon/internal/sampler/traceroute"
)

const shutdownTimeout time.Duration = 250 * time.Millisecond

type config struct {
	PeerID            string                `split_words:"true"`
	LatencyListenAddr string                `split_words:"true" default:":8080"`
	LatencySendAddrs  []string              `split_words:"true" default:":8081"`
	LatencyInterval   time.Duration         `split_words:"true" default:"1s"`
	TraceSendAddrs    []string              `split_words:"true" default:""`
	TraceInterval     time.Duration         `split_words:"true" default:"1s"`
	TraceMaxHops      int                   `split_words:"true" default:"12"`
	Metrics           []recorder.SampleType `split_words:"true" default:"rtt,hop_rtt,downstream_jitter,upstream_jitter,sent_packets,lost_packets"`
	MetricsAddr       string                `split_words:"true" default:""`
	LogLevel          slog.Level            `split_words:"true" default:"INFO"`
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

	if conf.MetricsAddr != "" {
		prometheus, err := recorder.NewPrometheus(conf.MetricsAddr, log)
		if err != nil {
			return err
		}
		group = append(group, prometheus)
		recorders = append(recorders, prometheus.DefaultRecorders()...)
	}

	chain := recorder.Chain(recorders...)

	peer, err := p2platency.NewPeer(
		p2platency.WithID(conf.PeerID),
		p2platency.WithListenAddress(conf.LatencyListenAddr),
		p2platency.WithSendAddresses(conf.LatencySendAddrs...),
		p2platency.WithInterval(conf.LatencyInterval),
		p2platency.WithRecorder(chain),
		p2platency.WithServerReflectionEnabled(true), // TODO: make controllable.
		p2platency.WithLog(log),
	)
	if err != nil {
		return err
	}
	group = append(group, peer)

	for _, addr := range conf.TraceSendAddrs {
		traceRouteSampler, err := traceroute.NewTraceRoute(traceroute.TraceRouteOptions{
			ID:       conf.PeerID,
			Address:  addr,
			MaxHops:  conf.TraceMaxHops,
			Timeout:  conf.TraceInterval,
			Interval: conf.TraceInterval,
			Recorder: chain,
			Log:      log,
		})
		if err != nil {
			return err
		}
		group = append(group, traceRouteSampler)
	}

	if err := group.Run(ctx, shutdownTimeout, exitSignals...); err != nil {
		return err
	}

	return nil
}
