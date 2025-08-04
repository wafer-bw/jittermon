package store

import (
	_ "embed"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/wafer-bw/go-toolbox/funcopts"
	"github.com/wafer-bw/jittermon/internal/recorder"
)

const (
	trailingActiveDataCapacity int           = 3
	DefaultCapacity            int           = 20_000
	readTimeout                time.Duration = 1 * time.Second // TODO: make configurable.
	writeTimeout               time.Duration = 2 * time.Second // TODO: make configurable.
	idleTimeout                time.Duration = 5 * time.Second // TODO: make configurable.
)

var templateFiles []string = []string{
	"./public/views/html.html",
	"./public/views/index.html",
}

type Templates struct {
	Index *template.Template
}

func (t *Templates) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.Index.ExecuteTemplate(w, name, data)
}

type AggRows []AggRow

func (a AggRows) Range(start time.Time, end time.Time) AggRows {
	if len(a) == 0 {
		return nil
	}

	var result []AggRow
	for _, row := range a {
		if row.Time.After(start) && row.Time.Before(end) {
			result = append(result, row)
		}
	}

	return result
}

func (a AggRows) MarshalJSON() ([]byte, error) {
	rows := make([]any, len(a)+1)
	rows[0] = []any{"Time", "Ping", "Jitter", "Packet Loss"}
	for i, row := range a {
		rows[i+1] = []any{
			row.Time.Format(time.DateTime),
			row.RTTMilliseconds,
			row.JitterMilliseconds,
			row.PacketLoss,
		}
	}

	return json.Marshal(rows)
}

type AggRow struct {
	Time               time.Time `json:"time"`
	PacketLoss         float64   `json:"packetLossPercentage"`
	RTTMilliseconds    float64   `json:"rttMilliseconds"`
	JitterMilliseconds float64   `json:"jitterMilliseconds"`
}

func (a AggRow) String() string {
	return fmt.Sprintf(
		"time: %s, packetLoss: %.2f%%, rttMilliseconds: %.2f, jitterMilliseconds: %.2f",
		a.Time.Format(time.RFC3339), a.PacketLoss, a.RTTMilliseconds, a.JitterMilliseconds,
	)
}

type Rows []Row

func (r Rows) Index(t time.Time) (int, bool) {
	for i, row := range r {
		if row.time.Equal(t) {
			return i, true
		}
	}

	return 0, false
}

type Row struct {
	time        time.Time
	lostPackets int
	sentPackets int
	rttCount    int
	rttSum      time.Duration
	jitterCount int
	jitterSum   time.Duration
}

func (r Row) String() string {
	return fmt.Sprintf(
		"time: %s, sentPackets: %d, lostPackets: %d, rttCount: %d, rttSum: %s, jitterCount: %d, jitterSum: %s",
		r.time.Format(time.RFC3339), r.sentPackets, r.lostPackets, r.rttCount, r.rttSum.String(), r.jitterCount, r.jitterSum.String(),
	)
}

func (r Row) Aggregate() AggRow {
	var packetLoss float64
	if r.sentPackets > 0 {
		//nolint:mnd // convert to percentage.
		packetLoss = float64(r.lostPackets) / float64(r.sentPackets) * 100
	}

	var rttMs float64
	if r.rttCount > 0 {
		//nolint:mnd // convert to milliseconds.
		rttMs = float64(r.rttSum.Microseconds()) / 1000 / float64(r.rttCount)
	}

	var jitterMs float64
	if r.jitterCount > 0 {
		//nolint:mnd // convert to milliseconds.
		jitterMs = float64(r.jitterSum.Microseconds()) / 1000 / float64(r.jitterCount)
	}

	return AggRow{
		Time:               r.time,
		PacketLoss:         packetLoss,
		RTTMilliseconds:    rttMs,
		JitterMilliseconds: jitterMs,
	}
}

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

	mu *sync.RWMutex

	aggregate  AggRows
	activeData Rows
}

func New(opts ...Option) (*Store, error) {
	s := &Store{
		capacity: DefaultCapacity,
		log:      slog.New(slog.DiscardHandler),
		mu:       &sync.RWMutex{},

		activeData: make(Rows, 0, trailingActiveDataCapacity),
	}

	if err := funcopts.Process(s, opts...); err != nil {
		return nil, err
	}

	s.log.Warn("mem store server temporarily disabled") // TODO: support embeding templates then add this back.

	s.server = &http.Server{
		Addr:         ":8083", // TODO: make configurable.
		Handler:      http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "not implemented", 500) }), // s.Router(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	s.aggregate = make(AggRows, 0, s.capacity)

	return s, nil
}

func (s *Store) Recorder(next recorder.Recorder) recorder.Recorder {
	return recorder.RecorderFunc(func(ctx context.Context, sample recorder.Sample) {
		defer next.Record(ctx, sample)

		s.mu.Lock()
		defer s.mu.Unlock()

		tick := sample.Time.Round(time.Second)

		if len(s.activeData) > trailingActiveDataCapacity {
			s.aggregate = append(s.aggregate, s.activeData[0].Aggregate())
			s.activeData = s.activeData[1:]
		}

		if len(s.aggregate) > s.capacity {
			s.aggregate = s.aggregate[1:]
		}

		activeIndex, ok := s.activeData.Index(tick)
		if !ok {
			s.activeData = append(s.activeData, Row{time: tick})
			activeIndex = len(s.activeData) - 1
		}

		data := s.activeData[activeIndex]
		duration, durationOk := sample.GetDuration()

		switch sample.Type {
		case recorder.SampleTypeSentPackets:
			data.sentPackets++
		case recorder.SampleTypeLostPackets:
			data.lostPackets++
		case recorder.SampleTypeRTT:
			if !durationOk {
				break
			}
			data.rttCount++
			data.rttSum += duration
		case recorder.SampleTypeRTTJitter:
			if !durationOk {
				break
			}
			data.jitterCount++
			data.jitterSum += duration
		case // ignored.
			recorder.SampleTypeDownstreamJitter,
			recorder.SampleTypeUpstreamJitter,
			recorder.SampleTypeHopRTT:
			break
		}

		s.activeData[activeIndex] = data
	})
}

func (s *Store) Router() *echo.Echo {
	e := echo.New()
	// e.Use(middleware.Logger())
	e.Renderer = &Templates{Index: template.Must(template.ParseFiles(templateFiles...))}
	e.Static("/static", "./public/static")

	e.GET("/", func(c echo.Context) error { return c.Render(http.StatusOK, "index", nil) })
	e.GET("/data", s.HandleChart)

	return e
}

func (s *Store) HandleChart(c echo.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().Round(time.Second)

	end := now
	endDuration, _ := time.ParseDuration(c.QueryParam("end"))
	if endDuration != 0 {
		end = end.Add(-endDuration)
	}
	if end.After(now) {
		end = now
	}

	start := end.Add(-15 * time.Minute)
	startDuration, _ := time.ParseDuration(c.QueryParam("start"))
	if startDuration != 0 {
		start = end.Add(-startDuration)
	}
	if start.After(end) {
		start = end.Add(-15 * time.Minute)
	}

	return c.JSON(http.StatusOK, s.aggregate.Range(start, end))
}

func (s Store) Start(ctx context.Context) error {
	s.log.Info("starting", "name", "data_server", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s Store) Stop(ctx context.Context) error {
	s.log.Debug("stopping prometheus server")
	return s.server.Shutdown(ctx)
}
