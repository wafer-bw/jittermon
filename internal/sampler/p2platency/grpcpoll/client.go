package grpcpoll

import (
	"context"

	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platency"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ p2platency.Poller = (*Client)(nil)

type Client struct {
	target  string
	options []grpc.DialOption

	client pollpb.PollServiceClient
	conn   *grpc.ClientConn

	startedCh chan struct{}
}

func NewClient(target string, opts ...grpc.DialOption) (*Client, error) {
	return &Client{
		target:    target,
		options:   opts,
		startedCh: make(chan struct{}),
	}, nil
}

func (c Client) Poll(ctx context.Context, r p2platency.PollRequest) (p2platency.PollResponse, error) {
	req := &pollpb.PollRequest{}
	req.SetId(r.ID)
	req.SetTimestamp(timestamppb.New(r.Timestamp))

	rsp, err := c.client.Poll(ctx, req)
	if err != nil {
		return p2platency.PollResponse{}, err
	}

	pollResponse := p2platency.PollResponse{ID: rsp.GetId()}

	jitterPb := rsp.GetJitter()
	if jitterPb == nil {
		return pollResponse, nil
	}

	jitter := jitterPb.AsDuration()
	pollResponse.Jitter = &jitter

	return pollResponse, nil
}

func (c *Client) Start() error {
	var err error
	c.conn, err = grpc.NewClient(c.target, c.options...)
	if err != nil {
		return err
	}

	c.client = pollpb.NewPollServiceClient(c.conn)

	// unblock [Client.Stop] in case it is called before the above finishes.
	close(c.startedCh)

	return nil
}

func (c *Client) Stop() error {
	<-c.startedCh // wait for [Client.Start] to finish
	return c.conn.Close()
}

func (c Client) Address() string {
	return c.target
}
