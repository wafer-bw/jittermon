package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/peer"
	"github.com/wafer-bw/jittermon/internal/recorder"
)

// TODO: switch to cobra for more ergonomic CLI.
var (
	listenAddr = flag.String("l", "", "address to listen on")
	sendAddrs  = flag.String("s", "", "comma separated addresses to send to")
	interval   = flag.Duration("i", 1*time.Second, "polling interval")
	logLevel   = flag.String("L", "INFO", "log level")
	write      = flag.Bool("w", false, "write to file(s)")
	slogLevel  slog.Level
)

func init() {
	flag.Parse()

	if *listenAddr == "" {
		fmt.Println("-l flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if *sendAddrs == "" {
		fmt.Println("-s flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if err := slogLevel.UnmarshalText([]byte(*logLevel)); err != nil {
		fmt.Println("invalid log level")
	}
}

func main() {
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel}))

	shutdownTimeout := 250 * time.Millisecond
	exitSignals := []os.Signal{syscall.SIGINT, syscall.SIGTERM}

	var jitterCSV, rttCSV peer.Recorder
	if *write {
		jitterCSV = recorder.CSV{}
		rttCSV = recorder.CSV{}
	}

	p, err := peer.NewPeer(uuid.New().String(), jitterCSV, rttCSV, log)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	group := graceful.Group{}

	addresses := strings.Split(*sendAddrs, ",")
	for _, addr := range addresses {
		group = append(group, &peer.Client{
			Addr:     addr,
			Poller:   p,
			Interval: *interval,
			Log:      log,
		})
	}

	group = append(group, &peer.Server{
		Addr:    *listenAddr,
		Handler: p,
		Log:     log,
	})

	if err := group.Run(ctx, shutdownTimeout, exitSignals...); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}
