package deployment

import (
	"context"
	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/transport"
	"io"
	"os"
)

type grpcPeer struct{}

func (grpcPeer) Capabilities(ctx context.Context, endpoint string) (*api.Capabilities, error) {
	return transport.Discover(ctx, endpoint)
}
func (grpcPeer) Stage(ctx context.Context, endpoint string, spec *api.OperationSpec, path, token string, progress func(int64)) (*api.Operation, error) {
	c, e := transport.Dial(endpoint)
	if e != nil {
		return nil, e
	}
	defer c.Close()
	stream, e := api.NewPackageDeploymentClient(c).Stage(transport.WithToken(ctx, token))
	if e != nil {
		return nil, e
	}
	if e = stream.Send(&api.StageChunk{Spec: spec}); e != nil {
		return nil, e
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b := make([]byte, 256*1024)
	var sent int64
	for {
		n, e := f.Read(b)
		if n > 0 {
			if err := stream.Send(&api.StageChunk{Data: b[:n]}); err != nil {
				if err == io.EOF {
					return stream.CloseAndRecv()
				}
				return nil, err
			}
			sent += int64(n)
			progress(sent)
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
	}
	return stream.CloseAndRecv()
}
func (grpcPeer) Start(ctx context.Context, endpoint, id, token string) (*api.Operation, error) {
	c, e := transport.Dial(endpoint)
	if e != nil {
		return nil, e
	}
	defer c.Close()
	return api.NewPackageDeploymentClient(c).Start(transport.WithToken(ctx, token), &api.OperationQuery{Id: id})
}
func (grpcPeer) Get(ctx context.Context, endpoint, id, token string) (*api.Operation, error) {
	c, e := transport.Dial(endpoint)
	if e != nil {
		return nil, e
	}
	defer c.Close()
	return api.NewPackageDeploymentClient(c).Get(transport.WithToken(ctx, token), &api.OperationQuery{Id: id})
}

func (grpcPeer) Observe(ctx context.Context, endpoint, id, token string, progress func(*api.Operation) error) error {
	c, e := transport.Dial(endpoint)
	if e != nil {
		return e
	}
	defer c.Close()
	stream, e := api.NewPackageDeploymentClient(c).Watch(transport.WithToken(ctx, token), &api.OperationQuery{Id: id})
	if e != nil {
		return e
	}
	for {
		op, e := stream.Recv()
		if e != nil {
			return e
		}
		if e = progress(op); e != nil {
			return e
		}
		if op.State != "RUNNING" {
			return nil
		}
	}
}
