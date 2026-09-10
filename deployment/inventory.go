package deployment

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type inventoryAPI struct {
	api.UnimplementedPackageInventoryServer
	s *Server
}

func (a *inventoryAPI) List(ctx context.Context, _ *api.Empty) (*api.PackageCatalog, error) {
	if _, err := a.s.authorize(ctx, "MONITOR", nil); err != nil {
		return nil, err
	}
	return a.s.inventorySnapshot(ctx)
}
func (a *inventoryAPI) Watch(_ *api.Empty, stream grpc.ServerStreamingServer[api.PackageCatalog]) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		v, err := a.List(stream.Context(), &api.Empty{})
		if err != nil {
			return err
		}
		if err = stream.Send(v); err != nil {
			return err
		}
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-a.s.ctx.Done():
			return status.Error(codes.Unavailable, "Jupiter is stopping")
		case <-ticker.C:
		}
	}
}
func (s *Server) inventorySnapshot(ctx context.Context) (*api.PackageCatalog, error) {
	var entries []*api.PackageEntry
	var err error
	if s.Inventory != nil {
		entries, err = s.Inventory()
	} else {
		var targets []*api.Target
		targets, err = s.Targets()
		for _, t := range targets {
			entries = append(entries, &api.PackageEntry{Target: proto.Clone(t).(*api.Target)})
		}
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "read package registrations: "+err.Error())
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i].Target, entries[j].Target
		return a.Group+"/"+a.PackageId < b.Group+"/"+b.PackageId
	})
	catalog := &api.PackageCatalog{Packages: entries}
	spec, _ := proto.Marshal(catalog)
	// Share probes between viewers; never change legacy repository objects.
	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	if s.inventoryCache != nil && s.inventoryKey == string(spec) && time.Since(time.Unix(s.inventoryCache.ObservedAt, 0)) < 5*time.Second {
		return proto.Clone(s.inventoryCache).(*api.PackageCatalog), nil
	}
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	slots := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, entry := range entries {
		select {
		case slots <- struct{}{}:
		case <-check.Done():
			entry.State = "UNKNOWN"
			entry.Detail = "Health check budget exceeded"
			continue
		}
		wg.Add(1)
		go func(p *api.PackageEntry) { defer wg.Done(); defer func() { <-slots }(); probePackage(check, p) }(entry)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	catalog.ObservedAt = time.Now().Unix()
	s.inventoryCache = proto.Clone(catalog).(*api.PackageCatalog)
	s.inventoryKey = string(spec)
	return catalog, nil
}
func probePackage(ctx context.Context, p *api.PackageEntry) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	defer func() { p.CheckedAt = time.Now().Unix() }()
	caps, err := transport.Discover(ctx, p.Target.Endpoint)
	if err == nil {
		p.Transport = "gRPC"
		if caps.Server != "juno" || caps.PackageId != p.Target.PackageId {
			p.State = "MISMATCH"
			p.Detail = "Endpoint identity does not match registration"
			return
		}
		p.State = "ALIVE"
		p.Detail = "Juno API responded; application health is not checked"
		return
	}
	if errors.Is(err, transport.ErrLegacy) {
		p.Transport = "HTTP"
		address, e := transport.Address(p.Target.Endpoint)
		if e == nil {
			var req *http.Request
			req, e = http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address+"/package/health/v1", nil)
			if e == nil {
				client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
				var resp *http.Response
				resp, e = client.Do(req)
				if e == nil {
					resp.Body.Close()
					if resp.StatusCode/100 != 2 {
						e = fmt.Errorf("health check: HTTP %d", resp.StatusCode)
					}
				}
			}
		}
		if e == nil {
			p.State = "ALIVE"
			p.Detail = "Legacy Juno health endpoint responded"
			return
		}
		err = e
	}
	p.State = "UNREACHABLE"
	p.Detail = strings.ReplaceAll(err.Error(), "\n", " ")
}
