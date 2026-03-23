package otel

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

const (
	sampleRatio         float64 = 1.0
	instrumentationName string  = "github.com/wafer-bw/jittermon/internal/otel"

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

type otelConfig struct {
	serviceName        string
	serviceVersion     string
	serviceInstanceID  string
	sampleRatio        float64
	prometheusExporter *prometheus.Exporter
}

func Setup(ctx context.Context, id string) (shutdown func(context.Context) error, err error) {
	if id == "" {
		return shutdown, errors.New("id cannot be empty")
	}

	cfg := otelConfig{
		serviceInstanceID: id,
		sampleRatio:       sampleRatio,
	}

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

	var shutdownFuncs []func(context.Context) error
	shutdown = func(ctx context.Context) error {
		var errs error
		for _, fn := range shutdownFuncs {
			errs = errors.Join(errs, fn(ctx))
		}
		shutdownFuncs = nil
		return errs
	}

	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(prop)

	// trace
	traceExporter, err := otlptrace.New(ctx, otlptracehttp.NewClient())
	if err != nil {
		return shutdown, errors.Join(err, shutdown(ctx))
	}
	tracerProvider := trace.NewTracerProvider(
		trace.WithResource(resource),
		trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(cfg.sampleRatio))),
		trace.WithBatcher(traceExporter),
	)
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	// meter
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
