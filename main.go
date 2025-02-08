package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/net"
	"github.com/wafer-bw/jittermon/internal/peer"
	"github.com/wafer-bw/jittermon/internal/recorder"
)

var conf config

type config struct {
	PeerID      string        `split_words:"true" default:""`
	ListenAddr  string        `split_words:"true" default:":8080"`
	SendAddrs   []string      `split_words:"true" default:":8081"`
	MetricsAddr string        `split_words:"true" default:""`
	Interval    time.Duration `split_words:"true" default:"1s"`
	LogLevel    slog.Level    `split_words:"true" default:"INFO"`
	Write       bool          `split_words:"true" default:"false"`
}

func init() {
	envconfig.MustProcess("JITTERMON", &conf)
}

// TODO: better convergance of configured recorders based on provided flags.
func main() {
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: conf.LogLevel}))
	shutdownTimeout := 250 * time.Millisecond
	exitSignals := []os.Signal{syscall.SIGINT, syscall.SIGTERM}

	var jitterCSV, rttCSV peer.Recorder
	if conf.Write {
		jitterCSV = recorder.CSV{}
		rttCSV = recorder.CSV{}
	}
	_ = jitterCSV
	_ = rttCSV

	group := graceful.Group{}

	var prometheus peer.Recorder
	if conf.MetricsAddr != "" {
		r := &recorder.Prometheus{Addr: conf.MetricsAddr}
		prometheus = r
		group = append(group, r)
	}

	if conf.PeerID == "" {
		conf.PeerID = strings.Split(uuid.New().String(), "-")[1]
	}
	p, err := peer.NewPeer(conf.PeerID, prometheus, prometheus, log)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	if len(conf.SendAddrs) != 0 {
		for _, addr := range conf.SendAddrs {
			group = append(group, &net.Client{
				Addr:     addr,
				Poller:   p,
				Interval: conf.Interval,
				Log:      log,
			})
		}
	}

	if conf.ListenAddr != "" {
		group = append(group, &net.Server{
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
