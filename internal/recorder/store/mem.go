package store

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/wafer-bw/go-toolbox/funcopts"
	"github.com/wafer-bw/jittermon/internal/recorder"
)

const (
	DefaultCapacity int           = 1000
	readTimeout     time.Duration = 1 * time.Second // TODO: make configurable.
	writeTimeout    time.Duration = 2 * time.Second // TODO: make configurable.
	idleTimeout     time.Duration = 5 * time.Second // TODO: make configurable.
)

type Option func(*Store) error

func WithCapacity(capacity int) Option {
	return func(s *Store) error {
		if capacity <= 0 {
			return nil
		}
		s.capacity = capacity
		return nil
	}
}

func WithLogger(log *slog.Logger) Option {
	return func(s *Store) error {
		if log == nil {
			return nil
		}
		s.log = log
		return nil
	}
}

type Store struct {
	capacity int
	server   *http.Server
	log      *slog.Logger

	mu   *sync.RWMutex
	data map[recorder.SampleType][]recorder.Sample
}

func New(opts ...Option) (*Store, error) {
	s := &Store{
		capacity: DefaultCapacity,
		log:      slog.New(slog.DiscardHandler),
		mu:       &sync.RWMutex{},
		data:     map[recorder.SampleType][]recorder.Sample{},
	}

	server := &http.Server{
		Addr:         ":8083", // TODO: make configurable.
		Handler:      s,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	s.server = server

	if err := funcopts.Process(s, opts...); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) Recorder(next recorder.Recorder) recorder.Recorder {
	return recorder.RecorderFunc(func(ctx context.Context, sample recorder.Sample) {
		defer next.Record(ctx, sample)

		s.mu.Lock()
		defer s.mu.Unlock()

		if _, ok := s.data[sample.Type]; !ok {
			s.data[sample.Type] = make([]recorder.Sample, 0, s.capacity)
		} else if len(s.data[sample.Type]) >= s.capacity {
			s.data[sample.Type] = s.data[sample.Type][1:]
		}

		s.data[sample.Type] = append(s.data[sample.Type], sample)
	})
}

func (s *Store) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, err := json.Marshal(s.data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func (s Store) Start(ctx context.Context) error {
	s.log.Info("starting data server", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s Store) Stop(ctx context.Context) error {
	s.log.Debug("stopping prometheus server")
	return s.server.Shutdown(ctx)
}
