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
	address := ""
	if p, ok := peer.FromContext(ctx); ok {
		address, _, _ = net.SplitHostPort(p.Addr.String())
	}
	return resolveTarget(targets, q.PackageId, address, localAddresses())
}

// resolveTarget picks the package for a client that may omit -p. Without an
// explicit package it tries, in order: an endpoint on the client's address, a
// package on this host when the client is local (juno may register a wildcard
// such as 0.0.0.0), and the only registered package, as the HTTP API did.
func resolveTarget(targets []*api.Target, id, client string, local map[string]bool) (*api.Target, error) {
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
	found, err := matchTarget(targets, func(host string) bool { return host == client })
	if found != nil || err != nil {
		return found, err
	}
	if isLocalHost(client, local) {
		found, err = matchTarget(targets, func(host string) bool { return isWildcardHost(host) || isLocalHost(host, local) })
		if found != nil || err != nil {
			return found, err
		}
	}
	if len(targets) == 1 {
		return targets[0], nil
	}
	return nil, status.Error(codes.NotFound, "no package matches this client; specify -p host:package")
}

func matchTarget(targets []*api.Target, match func(host string) bool) (*api.Target, error) {
	var found *api.Target
	for _, t := range targets {
		u, e := url.Parse(t.Endpoint)
		if e == nil && match(u.Hostname()) {
			if found != nil {
				return nil, status.Error(codes.FailedPrecondition, "multiple packages match; specify -p host:package")
			}
			found = t
		}
	}
	return found, nil
}

func isWildcardHost(host string) bool {
	return host == "" || net.ParseIP(host).IsUnspecified()
}

func isLocalHost(host string, local map[string]bool) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || local[ip.String()])
}

// localAddresses lists this host's interface addresses so a client connecting
// through a LAN address is still recognised as local.
func localAddresses() map[string]bool {
	result := map[string]bool{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return result
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			result[n.IP.String()] = true
		}
	}
	return result
}
