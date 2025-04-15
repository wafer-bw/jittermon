package p2platency

import (
	"log/slog"
	"time"

	"github.com/wafer-bw/go-toolbox/graceful"
	"google.golang.org/grpc"
)

// export for testing.
func (p *Peer) SetGroup(g graceful.Group) {
	p.group = g
}

// export for testing.
func (p Peer) GetGroup() graceful.Group {
	return p.group
}

// export for testing.
func (p Peer) CloseStoppedCh() {
	close(p.stoppedCh)
}

// export for testing.
func (p Peer) GetID() string {
	return p.id
}

// export for testing.
func (p Peer) GetInterval() time.Duration {
	return p.interval
}

// export for testing.
func (p Peer) GetListenAddress() string {
	return p.listenAddress
}

// export for testing.
func (p Peer) GetSendAddresses() []string {
	return p.sendAddresses
}

// export for testing.
func (p Peer) GetLog() *slog.Logger {
	return p.log
}

// export for testing.
func (p Peer) GetRecorder() Recorder {
	return p.recorder
}

// export for testing.
func (p Peer) GetProto() string {
	return p.proto
}

// export for testing.
func (p Peer) GetDialOptions() []grpc.DialOption {
	return p.dialOptions
}

// export for testing.
func (p Peer) GetServerOptions() []grpc.ServerOption {
	return p.serverOptions
}

// export for testing.
func (p Peer) GetReflection() bool {
	return p.reflection
}
