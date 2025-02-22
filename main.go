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
	"github.com/wafer-bw/jittermon/internal/sampler"
)

const shutdownTimeout time.Duration = 250 * time.Millisecond

type config struct {
	PeerID      string                `split_words:"true"`
	ListenAddr  string                `split_words:"true" default:":8080"`
	SendAddrs   []string              `split_words:"true" default:":8081"`
	MetricsAddr string                `split_words:"true" default:""`
	Interval    time.Duration         `split_words:"true" default:"1s"`
	LogLevel    slog.Level            `split_words:"true" default:"INFO"`
	Metrics     []recorder.SampleType `split_words:"true" default:"rtt,downstream_jitter,upstream_jitter,sent_packets,lost_packets,hop_rtt"`
	Write       bool                  `split_words:"true" default:"false"`
}

func main() {
	ctx := context.Background()
	conf := &config{}
	envconfig.MustProcess("JITTERMON", conf)
	group := graceful.Group{}
	recorders := []recorder.ChainLink{recorder.MetricFilter(conf.Metrics...)}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: conf.LogLevel}))
	exitSignals := []os.Signal{syscall.SIGINT, syscall.SIGTERM}

	if conf.MetricsAddr != "" {
		prometheus, err := recorder.NewPrometheus(conf.MetricsAddr, log)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
		group = append(group, prometheus)
		recorders = append(recorders, prometheus.DefaultRecorders()...)
	}

	chain := recorder.Chain(recorders...)

	// TODO: do we need the defers or can we move init portions into start functions?

	// TODO: configure separately.
	latencyClientSampler, err := sampler.NewLatencyClient(sampler.LatencyClientOptions{
		ID:       conf.PeerID,
		Address:  conf.SendAddrs[0],
		Interval: conf.Interval,
		Recorder: chain,
		Log:      log,
	})
	if err != nil {
		log.Error(err.Error())
		return // TODO: put this in function so we can return proper exit code.
	}
	defer latencyClientSampler.Stop(ctx) // TODO: will this panic if it ends up getting called twice?
	group = append(group, latencyClientSampler)

	// TODO: configure separately.
	latencyServerSampler, err := sampler.NewLatencyServer(sampler.LatencyServerOptions{
		ID:       conf.PeerID,
		Address:  conf.ListenAddr,
		Recorder: chain,
		Log:      log,
	})
	if err != nil {
		log.Error(err.Error())
		return // TODO: put this in function so we can return proper exit code.
	}
	defer latencyServerSampler.Stop(ctx) // TODO: will this panic if it ends up getting called twice?
	group = append(group, latencyServerSampler)

	// TODO: configure separately.
	traceRouteSampler, err := sampler.NewTraceRoute(sampler.TraceRouteOptions{
		ID:       conf.PeerID,
		Address:  conf.SendAddrs[0],
		MaxHops:  12,                // TODO: configure.
		Timeout:  conf.Interval * 4, // TODO: configure.
		Interval: conf.Interval * 4, // TODO configure separately.
		Recorder: chain,
		Log:      log,
	})
	if err != nil {
		log.Error(err.Error())
		return // TODO: put this in function so we can return proper exit code.
	}
	defer traceRouteSampler.Stop(ctx) // TODO: will this panic if it ends up getting called twice?
	group = append(group, traceRouteSampler)

	if err := group.Run(ctx, shutdownTimeout, exitSignals...); err != nil {
		log.Error(err.Error())
		return // TODO: put this in function so we can return proper exit code.
	}
}
