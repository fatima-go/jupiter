package deployment

import (
	"context"
	"errors"
	"github.com/fatima-go/fatima-opm/api"
	"github.com/fatima-go/fatima-opm/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"time"
)

type registryProxy struct {
	api.UnimplementedProcessRegistryServer
	s *Server
}

func (a *registryProxy) connect(ctx context.Context, id, role string) (*grpc.ClientConn, context.Context, error) {
	if _, err := a.s.authorize(ctx, role, nil); err != nil {
		return nil, nil, err
	}
	if id == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "package_id is required")
	}
	t, err := (&routingAPI{s: a.s}).Resolve(ctx, &api.PackageQuery{PackageId: id})
	if err != nil {
		return nil, nil, err
	}
	check, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	caps, err := transport.Discover(check, t.Endpoint)
	if errors.Is(err, transport.ErrLegacy) {
		return nil, nil, status.Error(codes.Unimplemented, "Juno does not advertise roproc")
	}
	if err != nil {
		return nil, nil, err
	}
	if caps.Server != "juno" || caps.PackageId != t.PackageId {
		return nil, nil, status.Error(codes.FailedPrecondition, "Juno identity does not match the registration")
	}
	if !transport.Supports(caps, "roproc") {
		if transport.Supports(caps, "control_unavailable") || transport.Supports(caps, "unavailable") {
			return nil, nil, status.Error(codes.Unavailable, "Juno control APIs are unavailable")
		}
		return nil, nil, status.Error(codes.Unimplemented, "Juno does not advertise roproc")
	}
	c, err := transport.Dial(t.Endpoint)
	return c, transport.WithToken(ctx, transport.Token(ctx)), err
}
func (a *registryProxy) Catalog(ctx context.Context, q *api.RegistryQuery) (*api.RegistryCatalog, error) {
	c, f, err := a.connect(ctx, q.PackageId, "MONITOR")
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return api.NewProcessRegistryClient(c).Catalog(f, q)
}
func (a *registryProxy) Preview(ctx context.Context, q *api.RegistryRequest) (*api.RegistryPlan, error) {
	c, f, err := a.connect(ctx, q.PackageId, "MONITOR")
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return api.NewProcessRegistryClient(c).Preview(f, q)
}
func (a *registryProxy) Apply(ctx context.Context, q *api.RegistryRequest) (*api.ControlOperation, error) {
	c, f, err := a.connect(ctx, q.PackageId, "OPERATOR")
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return api.NewProcessRegistryClient(c).Apply(f, q)
}
func (a *registryProxy) Get(ctx context.Context, q *api.RegistryOperationQuery) (*api.ControlOperation, error) {
	c, f, err := a.connect(ctx, q.PackageId, "MONITOR")
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return api.NewProcessRegistryClient(c).Get(f, q)
}
func (a *registryProxy) Watch(q *api.RegistryOperationQuery, stream grpc.ServerStreamingServer[api.ControlOperation]) error {
	c, f, err := a.connect(stream.Context(), q.PackageId, "MONITOR")
	if err != nil {
		return err
	}
	defer c.Close()
	s, err := api.NewProcessRegistryClient(c).Watch(f, q)
	if err != nil {
		return err
	}
	for {
		v, err := s.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err = stream.Send(v); err != nil {
			return err
		}
	}
}
