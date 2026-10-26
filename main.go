package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/wafer-bw/jittermon/internal/grpcptp"
	"github.com/wafer-bw/jittermon/internal/otel"
	"github.com/wafer-bw/jittermon/internal/udpptx"
	"golang.org/x/sync/errgroup"
)

type config struct {
	ID              string                   `envconfig:"ID"` // defaults to [os.Hostname].
	PTXAddrs        []string                 `envconfig:"PTX_ADDRS" default:""`
	PTXInterval     time.Duration            `envconfig:"PTX_INTERVAL" default:"1s"`
	PTPListenAddr   string                   `envconfig:"PTP_LISTEN_ADDR" default:""`
	PTPSendAddrs    []string                 `envconfig:"PTP_SEND_ADDRS" default:""`
	PTPInterval     time.Duration            `envconfig:"PTP_INTERVAL" default:"1s"`
	MetricsServer   otel.MetricsServerConfig `envconfig:"METRICS_SERVER"`
	LogLevel        slog.Level               `envconfig:"LOG_LEVEL" default:"INFO"`
	ShutdownTimeout time.Duration            `envconfig:"SHUTDOWN_TIMEOUT" default:"5s"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config{
		MetricsServer: otel.MetricsServerConfig{StoppingCh: make(chan struct{})},
	}

	if err := envconfig.Process("JITTERMON", &cfg); err != nil {
		slog.Default().ErrorContext(ctx, err.Error())
		os.Exit(1)
	}

	if cfg.ID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			slog.Default().ErrorContext(ctx, "JITTERMON_ID not set and unable to determine hostname", "err", err)
			os.Exit(1)
		}
		cfg.ID = hostname
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))

	if err := run(ctx, log, cfg); err != nil && ctx.Err() == nil {
		log.ErrorContext(ctx, err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger, cfg config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	otelShutdown, err := otel.Setup(ctx, cfg.ID)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
		defer cancel()
		if err := otelShutdown(shutdownCtx); err != nil {
			log.ErrorContext(ctx, "otel shutdown failed", "err", err)
		}
	}()

	eg, ctx := errgroup.WithContext(ctx)

	if cfg.MetricsServer.Address != "" {
		eg.Go(func() error { return otel.StartMetricsServer(ctx, log, cfg.MetricsServer) })
	}

	for _, addr := range cfg.PTXAddrs {
		client := udpptx.Client{
			ID:                 cfg.ID,
			Address:            addr,
			Interval:           cfg.PTXInterval,
			SentPacketsCounter: otel.SentPacketsCounter,
			LostPacketsCounter: otel.LostPacketsCounter,
			PingHistogram:      otel.PingHistogram,
			JitterHistogram:    otel.JitterHistogram,
			Log:                log,
		}
		eg.Go(func() error { return client.Start(ctx) })
	}

	for _, addr := range cfg.PTPSendAddrs {
		client := grpcptp.Client{
			ID:                      cfg.ID,
			Address:                 addr,
			Interval:                cfg.PTPInterval,
			SentPacketsCounter:      otel.SentPacketsCounter,
			LostPacketsCounter:      otel.LostPacketsCounter,
			PingHistogram:           otel.PingHistogram,
			UpstreamJitterHistogram: otel.UpstreamJitterHistogram,
			Log:                     log,
		}
		eg.Go(func() error { return client.Start(ctx) })
	}

	if cfg.PTPListenAddr != "" {
		server := grpcptp.Server{
			ID:                        cfg.ID,
			Address:                   cfg.PTPListenAddr,
			DownstreamJitterHistogram: otel.DownstreamJitterHistogram,
			Log:                       log,
		}
		eg.Go(func() error { return server.Start(ctx) })
	}

	return eg.Wait()
}
