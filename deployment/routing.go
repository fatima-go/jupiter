package deployment

import (
	"context"
	"github.com/fatima-go/fatima-core/opm/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"net"
	"net/url"
	"strings"
)

type routingAPI struct {
	api.UnimplementedRoutingServer
	s *Server
}

func (a *routingAPI) Resolve(ctx context.Context, q *api.PackageQuery) (*api.Target, error) {
	if _, err := a.s.authorize(ctx, "MONITOR", nil); err != nil {
		return nil, err
	}
	targets, err := a.s.Targets()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	id := q.PackageId
	if id != "" && !strings.Contains(id, ":") {
		id += ":default"
	}
	if id != "" {
		for _, t := range targets {
			if strings.EqualFold(t.PackageId, id) {
				return t, nil
			}
		}
		return nil, status.Error(codes.NotFound, "package not registered: "+id)
	}
	address := ""
	if p, ok := peer.FromContext(ctx); ok {
		address, _, _ = net.SplitHostPort(p.Addr.String())
	}
	var found *api.Target
	for _, t := range targets {
		u, e := url.Parse(t.Endpoint)
		if e == nil && u.Hostname() == address {
			if found != nil {
				return nil, status.Error(codes.FailedPrecondition, "multiple packages match; specify -p host:package")
			}
			found = t
		}
	}
	if found != nil {
		return found, nil
	}
	return nil, status.Error(codes.NotFound, "no package matches this client; specify -p host:package")
}
