package sampler

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/durationpb"
)

const maxConnectionIdle time.Duration = 5 * time.Minute

type LatencyServer struct {
	id             string
	addr           string
	server         *grpc.Server
	requestBuffers jitter.HostPacketBuffers
	recorder       rec.Recorder
	log            *slog.Logger

	pollpb.UnimplementedPollServiceServer
}

type LatencyServerOptions struct {
	ID       string
	Address  string
	Recorder rec.Recorder
	Log      *slog.Logger
}

func NewLatencyServer(options LatencyServerOptions) (LatencyServer, error) {
	server := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: maxConnectionIdle}),
	)

	ls := LatencyServer{
		id:             options.ID,
		addr:           options.Address,
		server:         server,
		requestBuffers: jitter.NewHostPacketBuffers(),
		recorder:       options.Recorder,
		log:            slog.New(slog.DiscardHandler),
	}

	if options.Log != nil {
		ls.log = options.Log
	}

	return ls, nil
}

func (ls LatencyServer) Poll(ctx context.Context, req *pollpb.PollRequest) (*pollpb.PollResponse, error) {
	now := time.Now()
	resp := &pollpb.PollResponse{}
	resp.SetId(ls.id)

	srcID := req.GetId()
	if srcID == "" {
		return nil, fmt.Errorf("id is required")
	}

	sentAtPb := req.GetTimestamp()
	if sentAtPb == nil {
		return nil, fmt.Errorf("timestamp is required")
	}
	sentAt := sentAtPb.AsTime()

	ls.requestBuffers.Sample(srcID, jitter.Packet{S: sentAt, R: now})
	jitter, ok := ls.requestBuffers.Jitter(srcID)
	if !ok {
		return resp, nil
	}
	resp.SetJitter(durationpb.New(jitter))

	labels := rec.Labels{{K: "src", V: srcID}, {K: "dst", V: ls.id}}
	ls.recorder.Record(ctx, rec.Sample{Time: now, Type: rec.SampleTypeDownstreamJitter, Val: jitter, Labels: labels})

	return resp, nil
}

func (ls LatencyServer) Start(ctx context.Context) error {
	pollpb.RegisterPollServiceServer(ls.server, ls)
	reflection.Register(ls.server)

	listener, err := net.Listen("tcp", ls.addr)
	if err != nil {
		return err
	}
	// TODO: is it listening now or only after Serve is called?
	defer listener.Close() // TODO: does this need to be closed in Stop?

	ls.log.Info("starting gRPC server", "addr", listener.Addr())

	return ls.server.Serve(listener)
}

func (ls LatencyServer) Stop(ctx context.Context) error {
	ls.log.Info("stopping server", "addr", ls.addr)

	ok := make(chan struct{})
	go func() {
		ls.server.GracefulStop()
		close(ok)
	}()

	select {
	case <-ok:
		return nil
	case <-ctx.Done():
		ls.server.Stop()
		return ctx.Err()
	}
}
