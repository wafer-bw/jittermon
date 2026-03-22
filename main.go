package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/go-toolbox/probe"
	"github.com/wafer-bw/jittermon/internal/grpcptp"
	"github.com/wafer-bw/jittermon/internal/otel"
	"github.com/wafer-bw/jittermon/internal/udpptx"
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
	httpServer := setupHTTP(ctx, log, cfg.HTTP)

	otelShutdown, err := otel.Setup(ctx, cfg.ID)
	if err != nil {
		return err
	}
	defer otelShutdown.StopFunc(ctx)

	group := graceful.Group{httpServer, otelShutdown}

	for _, addr := range cfg.PTXAddrs {
		udpptxClient, err := udpptx.New(cfg.ID, addr,
			udpptx.WithInterval(cfg.PTXInterval),
			udpptx.WithLog(log),
		)
		if err != nil {
			return fmt.Errorf("failed to create udpptx client for %s: %w", addr, err)
		}
		group = append(group, udpptxClient)
	}

	for _, addr := range cfg.PTPSendAddrs {
		grpcptpClient, err := grpcptp.NewClient(cfg.ID, addr,
			grpcptp.WithClientInterval(cfg.PTPInterval),
			grpcptp.WithClientLog(log),
		)
		if err != nil {
			return fmt.Errorf("failed to create grpcptp client for %s: %w", addr, err)
		}
		group = append(group, grpcptpClient)
	}

	if cfg.PTPListenAddr != "" {
		grpcptpServer, err := grpcptp.NewServer(cfg.ID, cfg.PTPListenAddr,
			grpcptp.WithServerLog(log),
		)
		if err != nil {
			return fmt.Errorf("failed to create grpcptp server for %s: %w", cfg.PTPListenAddr, err)
		}
		group = append(group, grpcptpServer)
	}

	if err := group.Run(ctx,
		graceful.WithStopSignals(syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM),
		graceful.WithStopTimeout(cfg.ShutdownTimeout),
		graceful.WithStoppingCh(cfg.HTTP.StoppingCh),
	); err != nil {
		return err
	}

	return nil
}

func setupHTTP(ctx context.Context, log *slog.Logger, cfg httpConfig) graceful.RunnerType {
	name := "http server"

	maxBytes := func(next http.Handler) http.Handler {
		return http.MaxBytesHandler(next, cfg.MaxBodyBytes)
	}
	timeout := func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, cfg.HandlerTimeout, "service timeout")
	}
	readiness := func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-cfg.StoppingCh:
			return errors.New("service is stopping")
		default:
			return nil
		}
	}

	startupProbes := probe.Group{}
	livenessProbes := probe.Group{}
	readinessProbes := probe.Group{"server": probe.ProberFunc(readiness)}

	chainBeforeMux := []func(http.Handler) http.Handler{maxBytes, timeout}
	chainAfterMux := []func(http.Handler) http.Handler{}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("GET /probes/startup", startupProbes)
	mux.Handle("GET /probes/liveness", livenessProbes)
	mux.Handle("GET /probes/readiness", readinessProbes)

	s := &http.Server{
		Addr:           cfg.Address,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		IdleTimeout:    cfg.IdleTimeout,
		Handler:        use(use(mux, chainAfterMux...), chainBeforeMux...),
	}

	log.InfoContext(ctx, "starting", "name", name, "address", cfg.Address)

	return graceful.RunnerType{
		StartFunc: func(ctx context.Context) error {
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("http server error: %w", err)
			}
			return nil
		},
		StopFunc: func(ctx context.Context) error {
			if err := s.Shutdown(ctx); err != nil {
				return fmt.Errorf("http server graceful shutdown failed: %w", err)
			}
			return nil
		},
	}
}

// use wraps an [http.Handler] with the provided middlewares.
func use(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}
