package main

import (
	"context"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/comms"
	"github.com/wafer-bw/jittermon/internal/peer"
	"github.com/wafer-bw/jittermon/internal/recorder"
)

const shutdownTimeout time.Duration = 250 * time.Millisecond

type config struct {
	PeerID      string                `split_words:"true"`
	ListenAddr  string                `split_words:"true" default:":8080"`
	SendAddrs   []string              `split_words:"true" default:":8081"`
	MetricsAddr string                `split_words:"true" default:""`
	Interval    time.Duration         `split_words:"true" default:"1s"`
	LogLevel    slog.Level            `split_words:"true" default:"INFO"`
	Metrics     []recorder.SampleType `split_words:"true" default:"rtt,downstream_jitter,upstream_jitter,sent_packets,lost_packets"`
	Write       bool                  `split_words:"true" default:"false"`
}

func main() {
	ctx := context.Background()
	conf := &config{}
	envconfig.MustProcess("JITTERMON", conf)
	group := graceful.Group{}
	recorders := []func(recorder.Recorder) recorder.Recorder{recorder.MetricFilter(conf.Metrics...)}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: conf.LogLevel}))
	exitSignals := []os.Signal{syscall.SIGINT, syscall.SIGTERM}

	if conf.MetricsAddr != "" {
		prometheus := recorder.NewPrometheus(conf.MetricsAddr, log)
		group = append(group, prometheus)
		recorders = append(recorders, prometheus.DefaultRecorders()...)
	}

	p, err := peer.NewPeer(
		peer.WithID(conf.PeerID),
		peer.WithLogger(log),
		peer.WithRecorders(recorders...),
	)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	if len(conf.SendAddrs) != 0 {
		for _, addr := range conf.SendAddrs {
			group = append(group, &comms.Client{
				Addr:     addr,
				Poller:   p,
				Interval: conf.Interval,
				Log:      log,
			})
		}
	}

	if conf.ListenAddr != "" {
		group = append(group, &comms.Server{
			Addr:    conf.ListenAddr,
			Handler: p,
			Log:     log,
		})
	}

	if err := group.Run(ctx, shutdownTimeout, exitSignals...); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}
