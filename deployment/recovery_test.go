package deployment

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/transport"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type testPeer struct {
	mu        sync.Mutex
	op        *api.Operation
	starts    int
	loseStart bool
}

type capabilityPeer struct {
	testPeer
	caps *api.Capabilities
	err  error
}

func (p *capabilityPeer) Capabilities(context.Context, string) (*api.Capabilities, error) {
	return p.caps, p.err
}

func TestOnlyExplicitLegacyDiscoveryAllowsFallback(t *testing.T) {
	for _, tc := range []struct {
		name   string
		caps   *api.Capabilities
		err    error
		legacy bool
	}{
		{name: "legacy", err: transport.ErrLegacy, legacy: true},
		{name: "network failure", err: status.Error(codes.Unavailable, "offline")},
		{name: "storage failure", caps: &api.Capabilities{Server: "juno", Features: []string{"unavailable"}}},
		{name: "partial API", caps: &api.Capabilities{Server: "juno", PackageId: "host:default"}},
		{name: "wrong server", caps: &api.Capabilities{Server: "jupiter", PackageId: "host:default"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testServer(t, t.TempDir(), &capabilityPeer{caps: tc.caps, err: tc.err})
			targets, e := s.targets(context.Background(), "backend")
			if e != nil || len(targets) != 1 || targets[0].Supported || targets[0].Legacy != tc.legacy {
				t.Fatalf("incorrect fallback decision: targets=%v error=%v", targets, e)
			}
			a := seedArtifact(t, s)
			endpoint := &rolloutAPI{s: s}
			if _, e = endpoint.Create(userContext(s), &api.CreateRollout{RequestId: "blocked", ArtifactId: a.Id, Group: "backend", FirstPackageId: "host:default"}); status.Code(e) != codes.FailedPrecondition {
				t.Fatalf("unsupported target accepted: %v", e)
			}
			plans, e := endpoint.List(userContext(s), &api.Empty{})
			if e != nil || len(plans.Rollouts) != 0 {
				t.Fatalf("rejected deployment persisted: %v %v", plans, e)
			}
		})
	}
}

func (p *testPeer) Capabilities(context.Context, string) (*api.Capabilities, error) {
	return &api.Capabilities{Server: "juno", ApiVersion: 2, PackageId: "host:default", Platform: "darwin_arm64", Features: []string{"deployment"}}, nil
}
func (p *testPeer) Stage(_ context.Context, _ string, s *api.OperationSpec, _, _ string, progress func(int64)) (*api.Operation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.op = &api.Operation{Id: s.Id, ArtifactId: s.ArtifactId, Sha256: s.Sha256, PackageId: s.PackageId, Process: s.Process, State: "STAGED"}
	return proto.Clone(p.op).(*api.Operation), nil
}
func (p *testPeer) Start(context.Context, string, string, string) (*api.Operation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.op.State == "STAGED" {
		p.starts++
		p.op.State = "SUCCEEDED"
	}
	if p.loseStart {
		p.loseStart = false
		return nil, status.Error(codes.Unavailable, "lost response after acceptance")
	}
	return proto.Clone(p.op).(*api.Operation), nil
}
func (p *testPeer) Get(context.Context, string, string, string) (*api.Operation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.op == nil {
		return nil, status.Error(codes.NotFound, "missing")
	}
	return proto.Clone(p.op).(*api.Operation), nil
}

func testServer(t *testing.T, dir string, peer Peer) *Server {
	t.Helper()
	s, e := New(dir, func(string, string) (string, error) { return "OPERATOR", nil }, func() ([]*api.Target, error) {
		return []*api.Target{{PackageId: "host:default", Group: "backend", Endpoint: "http://localhost:9180"}}, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	s.Peer = peer
	t.Cleanup(s.Close)
	return s
}
func userContext(s *Server) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(transport.TokenHeader, s.sign(claims{Username: "test", Role: "OPERATOR", ExpiresAt: s.Now().Add(time.Hour).Unix()})))
}
func seedArtifact(t *testing.T, s *Server) *api.Artifact {
	t.Helper()
	a := &api.Artifact{Id: "a_test", Process: "example", Size: 1, Sha256: "abc", UploadedAt: s.Now().Unix(), ExpiresAt: s.Now().Add(24 * time.Hour).Unix()}
	var db artifactDB
	if e := s.Store.Update("artifacts", &db, func() error {
		db.Records = map[string]*artifactRecord{a.Id: {Artifact: a, RequestID: "seed"}}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(s.artifactPath(a.Id), []byte("x"), 0600)
	return a
}
func waitState(t *testing.T, s *Server, id, want string) *api.Rollout {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		p, e := s.getRollout(id)
		if e != nil {
			t.Fatal(e)
		}
		if p.State == want {
			return p
		}
		time.Sleep(10 * time.Millisecond)
	}
	p, _ := s.getRollout(id)
	t.Fatalf("wanted %s: %v", want, p)
	return nil
}

func TestLostStartResponseRestartReconcilesWithoutReplay(t *testing.T) {
	peer := &testPeer{loseStart: true}
	dir := t.TempDir()
	s := testServer(t, dir, peer)
	a := seedArtifact(t, s)
	api1 := &rolloutAPI{s: s}
	p, e := api1.Create(userContext(s), &api.CreateRollout{RequestId: "once", ArtifactId: a.Id, Group: "backend", FirstPackageId: "host:default"})
	if e != nil {
		t.Fatal(e)
	}
	s.tick()
	p = waitState(t, s, p.Id, "ATTENTION")
	s.Close()
	replacement := testServer(t, dir, peer)
	api2 := &rolloutAPI{s: replacement}
	if _, e = api2.Act(userContext(replacement), &api.RolloutAction{Id: p.Id, Action: "resume", ExpectedRevision: p.Revision}); e != nil {
		t.Fatal(e)
	}
	replacement.tick()
	waitState(t, replacement, p.Id, "SUCCEEDED")
	peer.mu.Lock()
	defer peer.mu.Unlock()
	if peer.starts != 1 {
		t.Fatal("accepted operation was repeated")
	}
}
func TestTwoJupitersShareArtifactIdentityAndExecutionLease(t *testing.T) {
	peer := &testPeer{}
	dir := t.TempDir()
	one := testServer(t, dir, peer)
	two := testServer(t, dir, peer)
	a := seedArtifact(t, one)
	if _, e := two.authorize(userContext(one), "OPERATOR", nil); e != nil {
		t.Fatal("shared signing identity failed", e)
	}
	p, e := (&rolloutAPI{s: one}).Create(userContext(one), &api.CreateRollout{RequestId: "shared", ArtifactId: a.Id, Group: "backend", FirstPackageId: "host:default"})
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); one.tick() }()
	go func() { defer wg.Done(); two.tick() }()
	wg.Wait()
	waitState(t, two, p.Id, "SUCCEEDED")
	peer.mu.Lock()
	defer peer.mu.Unlock()
	if peer.starts != 1 {
		t.Fatal("lease allowed duplicate execution")
	}
}
func TestExpiredArtifactBytesRemovedAndNoNewDispatch(t *testing.T) {
	peer := &testPeer{}
	s := testServer(t, t.TempDir(), peer)
	var clock atomic.Int64
	clock.Store(time.Now().Unix())
	s.Now = func() time.Time { return time.Unix(clock.Load(), 0) }
	a := seedArtifact(t, s)
	p, e := (&rolloutAPI{s: s}).Create(userContext(s), &api.CreateRollout{RequestId: "expiry", ArtifactId: a.Id, Group: "backend", FirstPackageId: "host:default"})
	if e != nil {
		t.Fatal(e)
	}
	clock.Add(86400)
	s.tick()
	waitState(t, s, p.Id, "EXPIRED")
	if _, e = os.Stat(s.artifactPath(a.Id)); !os.IsNotExist(e) {
		t.Fatal("expired bytes retained")
	}
	retained, e := s.getArtifact(a.Id)
	if e != nil || !retained.Expired {
		t.Fatal("expired metadata lost")
	}
	peer.mu.Lock()
	defer peer.mu.Unlock()
	if peer.starts != 0 {
		t.Fatal("expired deployment started")
	}
}
func TestDeploymentTicketCannotInvokeUserAPIsOrOtherTargets(t *testing.T) {
	s := testServer(t, t.TempDir(), &testPeer{})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(transport.TokenHeader, s.sign(claims{Username: "test", Role: "OPERATOR", ExpiresAt: s.Now().Add(time.Minute).Unix(), OperationID: "one", SHA256: "digest", PackageID: "host:default"})))
	if _, e := s.authorize(ctx, "MONITOR", nil); status.Code(e) != codes.PermissionDenied {
		t.Fatal("ticket accepted as user session")
	}
	for _, scope := range []*api.ValidateRequest{{OperationId: "two", Sha256: "digest", PackageId: "host:default"}, {OperationId: "one", Sha256: "changed", PackageId: "host:default"}, {OperationId: "one", Sha256: "digest", PackageId: "other:default"}} {
		if _, e := s.authorize(ctx, "OPERATOR", scope); status.Code(e) != codes.PermissionDenied {
			t.Fatal(fmt.Sprint("ticket escaped scope: ", scope))
		}
	}
}
