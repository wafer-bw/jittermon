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
	ListenAddr  string        `split_words:"true" default:":8080"`
	SendAddrs   []string      `split_words:"true" default:":8081"`
	MetricsAddr string        `split_words:"true" default:""`
	Interval    time.Duration `split_words:"true" default:"1s"`
	LogLevel    slog.Level    `split_words:"true" default:"INFO"`
	Write       bool          `split_words:"true" default:"false"`
}

// TODO: better convergance of configured recorders based on provided flags.
func main() {
	ctx := context.Background()
	conf := &config{}
	envconfig.MustProcess("JITTERMON", conf)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: conf.LogLevel}))
	exitSignals := []os.Signal{syscall.SIGINT, syscall.SIGTERM}

	var jitterCSV, rttCSV peer.Recorder
	if conf.Write {
		jitterCSV = recorder.NewCSV()
		rttCSV = recorder.NewCSV()
	}
	_ = jitterCSV
	_ = rttCSV

	group := graceful.Group{}

	var prometheus peer.Recorder
	if conf.MetricsAddr != "" {
		r := recorder.NewPrometheus(conf.MetricsAddr)
		prometheus = r
		group = append(group, r)
	}

	p, err := peer.NewPeer(conf.ListenAddr, prometheus, prometheus, prometheus, log)
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
