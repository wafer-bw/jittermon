package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wafer-bw/jittermon/internal/grpcptp"
	"github.com/wafer-bw/jittermon/internal/otel"
	"github.com/wafer-bw/jittermon/internal/udpptx"
	"golang.org/x/sync/errgroup"
)

type config struct {
	ID              string        `envconfig:"ID" default:""`
	PTXAddrs        []string      `envconfig:"PTX_ADDRS" default:""`
	PTXInterval     time.Duration `envconfig:"PTX_INTERVAL" default:"1s"`
	PTPListenAddr   string        `envconfig:"PTP_LISTEN_ADDR" default:""`
	PTPSendAddrs    []string      `envconfig:"PTP_SEND_ADDRS" default:""`
	PTPInterval     time.Duration `envconfig:"PTP_INTERVAL" default:"1s"`
	LogLevel        slog.Level    `envconfig:"LOG_LEVEL" default:"INFO"`
	ShutdownTimeout time.Duration `envconfig:"SHUTDOWN_TIMEOUT" default:"5s"`
	HTTP            httpConfig    `envconfig:"HTTP"`
}

type httpConfig struct {
	Address        string        `envconfig:"ADDR" default:":8082"`
	MaxHeaderBytes int           `envconfig:"MAX_HEADER_BYTES" default:"32000"` // 32KB
	MaxBodyBytes   int64         `envconfig:"MAX_BODY_BYTES" default:"512000"`  // 512KB
	HandlerTimeout time.Duration `envconfig:"HANDLER_TIMEOUT" default:"800ms"`
	ReadTimeout    time.Duration `envconfig:"READ_TIMEOUT" default:"300ms"`
	WriteTimeout   time.Duration `envconfig:"WRITE_TIMEOUT" default:"1s"`
	IdleTimeout    time.Duration `envconfig:"IDLE_TIMEOUT" default:"15s"`
	StoppingCh     chan struct{} `envconfig:"-"`
}

func main() {
	ctx := context.Background()

	cfg := config{HTTP: httpConfig{StoppingCh: make(chan struct{})}}
	envconfig.MustProcess("JITTERMON", &cfg)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))

	cfg.ID = strings.TrimSpace(cfg.ID)
	if cfg.ID == "" {
		log.Error("JITTERMON_ID environment variable is required")
		os.Exit(1)
	}

	if err := run(ctx, log, cfg); err != nil {
		log.Error(err.Error())
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
	defer otelShutdown(ctx)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error { return startHTTPServer(ctx, log, cfg.HTTP) })

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

func startHTTPServer(ctx context.Context, log *slog.Logger, cfg httpConfig) error {
	name := "http server"

	maxBytes := func(next http.Handler) http.Handler {
		return http.MaxBytesHandler(next, cfg.MaxBodyBytes)
	}
	timeout := func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, cfg.HandlerTimeout, "service timeout")
	}

	chainBeforeMux := []func(http.Handler) http.Handler{maxBytes, timeout}
	chainAfterMux := []func(http.Handler) http.Handler{}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())

	s := &http.Server{
		Addr:           cfg.Address,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		IdleTimeout:    cfg.IdleTimeout,
		Handler:        use(use(mux, chainAfterMux...), chainBeforeMux...),
	}

	log.InfoContext(ctx, "starting", "name", name, "address", cfg.Address)

	errCh := make(chan error)
	go func() {
		if err := s.ListenAndServe(); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.WarnContext(ctx, "context done, stopping", "name", name, "err", ctx.Err())
		return ctx.Err()
	case err := <-errCh:
		log.ErrorContext(ctx, "server failed", "name", name, "err", err)
		return err
	}
}

// use wraps an [http.Handler] with the provided middlewares.
func use(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}
