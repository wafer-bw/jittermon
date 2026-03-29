package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/otel"
)

func TestRun(t *testing.T) {
	t.Run("run successfully", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", srv.URL)
		t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")

		metricsPort, err := freeport.GetFreePort()
		require.NoError(t, err)
		metricsAddr := net.JoinHostPort("", strconv.Itoa(metricsPort))

		ptpPort, err := freeport.GetFreePort()
		require.NoError(t, err)
		ptpAddr := net.JoinHostPort("localhost", strconv.Itoa(ptpPort))

		ptxPort, err := freeport.GetFreePort()
		require.NoError(t, err)
		ptxAddr := net.JoinHostPort("localhost", strconv.Itoa(ptxPort))

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		log := slog.New(slog.NewTextHandler(io.Discard, nil))

		cfg := config{
			ID:            "test-instance",
			PTPListenAddr: ptpAddr,
			PTPSendAddrs:  []string{ptpAddr},
			PTPInterval:   250 * time.Millisecond,
			PTXAddrs:      []string{ptxAddr},
			PTXInterval:   250 * time.Millisecond,
			MetricsServer: otel.MetricsServerConfig{
				Address:        metricsAddr,
				MaxHeaderBytes: 32000,
				MaxBodyBytes:   512000,
				HandlerTimeout: 800 * time.Millisecond,
				ReadTimeout:    300 * time.Millisecond,
				WriteTimeout:   1 * time.Second,
				IdleTimeout:    15 * time.Second,
				StoppingCh:     make(chan struct{}),
			},
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- run(ctx, log, cfg)
		}()

		// Wait for metrics server to be ready.
		require.Eventually(t, func() bool {
			resp, err := http.Get("http://localhost:" + strconv.Itoa(metricsPort) + "/metrics")
			if err != nil {
				return false
			}
			resp.Body.Close()
			return resp.StatusCode == http.StatusOK
		}, 1*time.Second, 50*time.Millisecond)

		cancel()

		select {
		case err := <-errCh:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(5 * time.Second):
			t.Fatal("run did not return after context cancellation")
		}
	})

	t.Run("fail when missing id", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		log := slog.New(slog.NewTextHandler(io.Discard, nil))

		cfg := config{}
		err := run(ctx, log, cfg)
		require.Error(t, err)
	})
}
