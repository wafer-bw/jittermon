package otel_test

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/otel"
)

func TestSetup(t *testing.T) {
	t.Run("returns shutdown function on success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", srv.URL)
		t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")

		ctx := t.Context()

		shutdown, err := otel.Setup(ctx, "test-instance")
		require.NoError(t, err)
		require.NotNil(t, shutdown)

		err = shutdown(ctx)
		require.NoError(t, err)
	})

	t.Run("returns error when id is empty", func(t *testing.T) {
		_, err := otel.Setup(t.Context(), "")
		require.Error(t, err)
		require.Equal(t, err.Error(), "id cannot be empty")
	})
}

func TestStartMetricsServer(t *testing.T) {
	t.Parallel()

	t.Run("successful start", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))

		port, err := freeport.GetFreePort()
		require.NoError(t, err)
		addr := net.JoinHostPort("", strconv.Itoa(port))

		cfg := otel.MetricsServerConfig{}
		err = envconfig.Process("", &cfg)
		require.NoError(t, err)
		cfg.Address = addr

		errCh := make(chan error, 1)
		go func() {
			errCh <- otel.StartMetricsServer(ctx, logger, cfg)
		}()

		require.Eventually(t, func() bool {
			resp, err := http.Get("http://localhost:" + strconv.Itoa(port) + "/metrics")
			if err != nil {
				return false
			}
			_ = resp.Body.Close()
			return resp.StatusCode == http.StatusOK
		}, 1*time.Second, 10*time.Millisecond)

		cancel()
		err = <-errCh
		require.Error(t, err)
		require.Equal(t, context.Canceled.Error(), err.Error())
	})

	t.Run("logs context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))

		port, err := freeport.GetFreePort()
		require.NoError(t, err)
		addr := net.JoinHostPort("", strconv.Itoa(port))

		cfg := otel.MetricsServerConfig{}
		err = envconfig.Process("", &cfg)
		require.NoError(t, err)
		cfg.Address = addr

		errCh := make(chan error, 1)
		go func() {
			errCh <- otel.StartMetricsServer(ctx, logger, cfg)
		}()

		require.Eventually(t, func() bool {
			conn, err := net.Dial("tcp", "localhost:"+strconv.Itoa(port))
			if err != nil {
				return false
			}
			_ = conn.Close()
			return true
		}, 1*time.Second, 10*time.Millisecond)

		cancel()
		err = <-errCh
		require.Error(t, err)
		require.Equal(t, context.Canceled.Error(), err.Error())
		require.Contains(t, buf.String(), "context done, stopping")
	})

	t.Run("logs server errors", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))

		port, err := freeport.GetFreePort()
		require.NoError(t, err)
		addr := net.JoinHostPort("", strconv.Itoa(port))

		listener, err := net.Listen("tcp", addr) // occupy port so server start fails.
		require.NoError(t, err)
		defer listener.Close()

		cfg := otel.MetricsServerConfig{}
		err = envconfig.Process("", &cfg)
		require.NoError(t, err)
		cfg.Address = addr

		err = otel.StartMetricsServer(t.Context(), logger, cfg)
		require.Error(t, err)
		require.Contains(t, buf.String(), "server failed")
	})
}
