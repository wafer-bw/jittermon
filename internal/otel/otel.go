package otel

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

const (
	instrumentationName string = "github.com/wafer-bw/jittermon/internal/otel"

	SentPacketsMetricName      string = "sent.packets"
	LostPacketsMetricName      string = "lost.packets"
	PingMetricName             string = "ping"
	JitterMetricName           string = "jitter"
	UpstreamJitterMetricName   string = "upstream.jitter"
	DownstreamJitterMetricName string = "downstream.jitter"

	SourceLabelName      string = "src"
	DestinationLabelName string = "dst"
)

var (
	meter = otel.Meter(instrumentationName)

	SentPacketsCounter        metric.Int64Counter
	LostPacketsCounter        metric.Int64Counter
	PingHistogram             metric.Float64Histogram
	JitterHistogram           metric.Float64Histogram
	UpstreamJitterHistogram   metric.Float64Histogram
	DownstreamJitterHistogram metric.Float64Histogram
)

type MetricsServerConfig struct {
	Address        string        `envconfig:"ADDR" default:":8082"`
	MaxHeaderBytes int           `envconfig:"MAX_HEADER_BYTES" default:"32000"` // 32KB
	MaxBodyBytes   int64         `envconfig:"MAX_BODY_BYTES" default:"512000"`  // 512KB
	HandlerTimeout time.Duration `envconfig:"HANDLER_TIMEOUT" default:"800ms"`
	ReadTimeout    time.Duration `envconfig:"READ_TIMEOUT" default:"300ms"`
	WriteTimeout   time.Duration `envconfig:"WRITE_TIMEOUT" default:"1s"`
	IdleTimeout    time.Duration `envconfig:"IDLE_TIMEOUT" default:"15s"`
	StoppingCh     chan struct{} `envconfig:"-"`
}

type otelConfig struct {
	serviceName       string
	serviceVersion    string
	serviceInstanceID string
}

func init() {
	var err error

	SentPacketsCounter, err = meter.Int64Counter(
		SentPacketsMetricName,
		metric.WithDescription("Number of packets sent to peers"),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create sent packets counter: %v", err))
	}

	LostPacketsCounter, err = meter.Int64Counter(
		LostPacketsMetricName,
		metric.WithDescription("Number of packets sent to peers that were lost"),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create lost packets counter: %v", err))
	}

	PingHistogram, err = meter.Float64Histogram(
		PingMetricName,
		metric.WithDescription("Latency of ping packets sent to peers"),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create ping histogram: %v", err))
	}

	JitterHistogram, err = meter.Float64Histogram(
		JitterMetricName,
		metric.WithDescription("Interarrival jitter of packets sent to and received from external addresses, calculated as per RFC3550 Section 6.4.1"),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create jitter histogram: %v", err))
	}

	UpstreamJitterHistogram, err = meter.Float64Histogram(
		UpstreamJitterMetricName,
		metric.WithDescription("Interarrival jitter of packets sent to peers, calculated as per RFC3550 Section 6.4.1"),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create upstream jitter histogram: %v", err))
	}

	DownstreamJitterHistogram, err = meter.Float64Histogram(
		DownstreamJitterMetricName,
		metric.WithDescription("Interarrival jitter of packets received from peers, calculated as per RFC3550 Section 6.4.1"),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create downstream jitter histogram: %v", err))
	}
}

func Setup(ctx context.Context, id string) (shutdown func(context.Context) error, err error) {
	if id == "" {
		return shutdown, errors.New("id cannot be empty")
	}

	cfg := otelConfig{serviceInstanceID: id}

	if buildInfo, ok := debug.ReadBuildInfo(); ok && buildInfo != nil {
		serviceNameParts := strings.Split(buildInfo.Main.Path, "/")
		cfg.serviceName = serviceNameParts[len(serviceNameParts)-1]
		cfg.serviceVersion = buildInfo.Main.Version
	}

	if cfg.serviceName == "" {
		return shutdown, errors.New("unable to identify service name")
	} else if cfg.serviceVersion == "" {
		return shutdown, errors.New("unable to identify service version")
	} else if cfg.serviceInstanceID == "" {
		return shutdown, errors.New("unable to identify service instance ID")
	}

	resource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.serviceName),
		semconv.ServiceInstanceID(cfg.serviceInstanceID),
		semconv.ServiceVersion(cfg.serviceVersion),
	)

	// Although only one is added, if you ever use traces or otel logs you will
	// end up with multiple so this is left as a slice.
	//
	// See also:
	// https://github.com/grafana/docker-otel-lgtm/blob/main/examples/go/otel.go
	shutdownFuncs := make([]func(context.Context) error, 0, 1)
	shutdown = func(ctx context.Context) error {
		var errs error
		for _, fn := range shutdownFuncs {
			errs = errors.Join(errs, fn(ctx))
		}
		shutdownFuncs = nil
		return errs
	}

	meterExporter, err := prometheus.New()
	if err != nil {
		return shutdown, errors.Join(err, shutdown(ctx))
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource),
		sdkmetric.WithReader(meterExporter),
	)
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second)); err != nil {
		return shutdown, errors.Join(err, shutdown(ctx))
	}

	return shutdown, nil
}

func StartMetricsServer(ctx context.Context, log *slog.Logger, cfg MetricsServerConfig) error {
	name := "metrics server"

	maxBytes := func(next http.Handler) http.Handler {
		return http.MaxBytesHandler(next, cfg.MaxBodyBytes)
	}
	timeout := func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, cfg.HandlerTimeout, "service timeout")
	}

	chain := []func(http.Handler) http.Handler{maxBytes, timeout}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())

	s := &http.Server{
		Addr:           cfg.Address,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		IdleTimeout:    cfg.IdleTimeout,
		Handler:        use(mux, chain...),
	}

	log.InfoContext(ctx, "starting", "name", name, "address", cfg.Address)

	errCh := make(chan error, 1)
	go func() {
		if err := s.ListenAndServe(); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.WarnContext(ctx, "context done, stopping", "name", name, "err", ctx.Err())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err := s.Shutdown(shutdownCtx)
		return cmp.Or(err, ctx.Err())
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
