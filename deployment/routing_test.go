package deployment

import (
	"testing"

	"github.com/fatima-go/fatima-opm/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestResolveTarget(t *testing.T) {
	wildcard := &api.Target{PackageId: "mac:default", Endpoint: "http://0.0.0.0:9180/seed/"}
	remote := &api.Target{PackageId: "remote:default", Endpoint: "http://10.0.0.5:9180/seed/"}
	loopback := &api.Target{PackageId: "mac:second", Endpoint: "http://127.0.0.1:9181/seed/"}
	lan := &api.Target{PackageId: "mac:lan", Endpoint: "http://192.168.0.10:9180/seed/"}
	local := map[string]bool{"192.168.0.10": true}
	cases := []struct {
		name    string
		targets []*api.Target
		id      string
		client  string
		want    *api.Target
		code    codes.Code
	}{
		{"explicit package", []*api.Target{wildcard, remote}, "remote", "127.0.0.1", remote, codes.OK},
		{"explicit package not registered", []*api.Target{wildcard}, "other:default", "127.0.0.1", nil, codes.NotFound},
		{"exact client address", []*api.Target{wildcard, remote}, "", "10.0.0.5", remote, codes.OK},
		{"loopback client to wildcard endpoint", []*api.Target{wildcard, remote}, "", "127.0.0.1", wildcard, codes.OK},
		{"ipv6 loopback client", []*api.Target{wildcard, remote}, "", "::1", wildcard, codes.OK},
		{"lan client to wildcard endpoint", []*api.Target{wildcard, remote}, "", "192.168.0.10", wildcard, codes.OK},
		{"loopback client to lan endpoint", []*api.Target{lan, remote}, "", "127.0.0.1", lan, codes.OK},
		{"several local packages", []*api.Target{wildcard, loopback, remote}, "", "::1", nil, codes.FailedPrecondition},
		{"only package for remote client", []*api.Target{wildcard}, "", "10.0.0.9", wildcard, codes.OK},
		{"remote client without match", []*api.Target{wildcard, loopback}, "", "10.0.0.9", nil, codes.NotFound},
		{"no packages", nil, "", "127.0.0.1", nil, codes.NotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveTarget(c.targets, c.id, c.client, local)
			if status.Code(err) != c.code {
				t.Fatalf("code = %v, want %v (%v)", status.Code(err), c.code, err)
			}
			if got != c.want {
				t.Fatalf("target = %v, want %v", got, c.want)
			}
		})
	}
}
