package deployment

import (
	"context"
	"fmt"
	"strings"
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

type managedPeer struct {
	mu                  sync.Mutex
	ops                 map[string]*api.Operation
	starts              int
	fail, lose, offline bool
}

func (p *managedPeer) Capabilities(_ context.Context, endpoint string) (*api.Capabilities, error) {
	return &api.Capabilities{Server: "juno", PackageId: endpoint, Features: []string{"deployment", "deployment_cancel"}}, nil
}
func (p *managedPeer) Stage(_ context.Context, _ string, s *api.OperationSpec, _, _ string, _ func(int64)) (*api.Operation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ops == nil {
		p.ops = map[string]*api.Operation{}
	}
	if p.ops[s.Id] == nil {
		p.ops[s.Id] = &api.Operation{Id: s.Id, ArtifactId: s.ArtifactId, PackageId: s.PackageId, Sha256: s.Sha256, Process: s.Process, State: "STAGED"}
	}
	return proto.Clone(p.ops[s.Id]).(*api.Operation), nil
}
func (p *managedPeer) Start(_ context.Context, _, id, _ string) (*api.Operation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	op := p.ops[id]
	if op.State == "STAGED" {
		p.starts++
		op.State = "RUNNING"
		if p.fail {
			op.State = "FAILED"
			op.Error = "startup failed"
		}
	}
	if p.lose {
		p.lose = false
		return nil, status.Error(codes.Unavailable, "lost start response")
	}
	return proto.Clone(op).(*api.Operation), nil
}
func (p *managedPeer) Get(_ context.Context, _, id, _ string) (*api.Operation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.offline {
		return nil, status.Error(codes.Unavailable, "offline")
	}
	op := p.ops[id]
	if op == nil {
		return nil, status.Error(codes.NotFound, "missing")
	}
	return proto.Clone(op).(*api.Operation), nil
}
func (p *managedPeer) Cancel(_ context.Context, _ string, s *api.OperationSpec, _ string) (*api.Operation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.offline {
		return nil, status.Error(codes.Unavailable, "offline")
	}
	if p.ops == nil {
		p.ops = map[string]*api.Operation{}
	}
	op := p.ops[s.Id]
	if op == nil {
		op = &api.Operation{Id: s.Id, ArtifactId: s.ArtifactId, PackageId: s.PackageId, Sha256: s.Sha256, Process: s.Process, State: "STAGED"}
		p.ops[s.Id] = op
	}
	if op.State == "STAGED" {
		op.State = "CANCELLED"
	}
	return proto.Clone(op).(*api.Operation), nil
}
func managedFixture(t *testing.T, peer *managedPeer) (*Server, *api.Rollout, *api.ManagementCredential, *atomic.Int64) {
	t.Helper()
	s := testServer(t, t.TempDir(), peer)
	s.Targets = func() ([]*api.Target, error) {
		return []*api.Target{{PackageId: "a:default", Group: "backend", Endpoint: "a:default"}, {PackageId: "b:default", Group: "backend", Endpoint: "b:default"}}, nil
	}
	var clock atomic.Int64
	clock.Store(time.Now().UnixMilli())
	s.Now = func() time.Time { return time.UnixMilli(clock.Load()) }
	a := seedArtifact(t, s)
	_ = a
	m, err := (&managementAPI{s: s}).Open(userContext(s), &api.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	credential := &api.ManagementCredential{Id: m.Id, Token: m.Token}
	p, err := (&rolloutAPI{s: s}).Create(userContext(s), &api.CreateRollout{RequestId: "managed", ArtifactId: a.Id, Group: "backend", FirstPackageId: "a:default", Management: credential})
	if err != nil {
		t.Fatal(err)
	}
	return s, p, credential, &clock
}
func TestManagementExpiresAtTwentySecondsWithoutResurrection(t *testing.T) {
	s, p, c, clock := managedFixture(t, &managedPeer{})
	m := &managementAPI{s: s}
	wrong := proto.Clone(c).(*api.ManagementCredential)
	wrong.Token = "other"
	if _, err := m.Detach(userContext(s), wrong); status.Code(err) != codes.PermissionDenied {
		t.Fatal(err)
	}
	observer := metadata.NewIncomingContext(context.Background(), metadata.Pairs(transport.TokenHeader, s.sign(claims{Username: "observer", Role: "OPERATOR", ExpiresAt: s.Now().Add(time.Hour).Unix()})))
	if _, err := m.Heartbeat(observer, c); status.Code(err) != codes.PermissionDenied {
		t.Fatal(err)
	}
	clock.Add(19000)
	beat, err := m.Heartbeat(userContext(s), c)
	if err != nil || beat.State != "ACTIVE" || beat.Token != "" {
		t.Fatal(beat, err)
	}
	clock.Add(20000)
	beat, err = m.Heartbeat(userContext(s), c)
	if err != nil || beat.State != "OWNER_TIMEOUT" {
		t.Fatal(beat, err)
	}
	s.tick()
	result := waitState(t, s, p.Id, "CANCELLED")
	if result.BlocksDeployment || result.EndReason != "OWNER_TIMEOUT" {
		t.Fatal(result)
	}
	beat, err = m.Heartbeat(userContext(s), c)
	if err != nil || beat.State == "ACTIVE" {
		t.Fatal("resurrected", beat, err)
	}
}
func TestDetachDuringExecutionOnlyFinishesCurrentTarget(t *testing.T) {
	peer := &managedPeer{lose: true}
	s, p, c, clock := managedFixture(t, peer)
	s.tick()
	p = waitState(t, s, p.Id, "ATTENTION")
	if _, err := (&managementAPI{s: s}).Detach(userContext(s), c); err != nil {
		t.Fatal(err)
	}
	peer.mu.Lock()
	for _, op := range peer.ops {
		op.State = "SUCCEEDED"
	}
	peer.mu.Unlock()
	clock.Add(3000)
	s.tick()
	result := waitState(t, s, p.Id, "CANCELLED")
	if result.Targets[0].Operation.State != "SUCCEEDED" || result.Targets[1].Operation.State != "CANCELLED" {
		t.Fatal(result)
	}
	peer.mu.Lock()
	defer peer.mu.Unlock()
	if peer.starts != 1 {
		t.Fatalf("started remaining target: %d", peer.starts)
	}
}
func TestFailureIsTerminalAndStructuredConflictRecovers(t *testing.T) {
	peer := &managedPeer{fail: true}
	s, p, _, _ := managedFixture(t, peer)
	q := &api.CreateRollout{RequestId: "second", ArtifactId: p.Artifact.Id, Group: "backend", FirstPackageId: "a:default"}
	a := &rolloutAPI{s: s}
	_, err := a.Create(userContext(s), q)
	if status.Code(err) != codes.Aborted {
		t.Fatal(err)
	}
	d := status.Convert(err).Details()
	if len(d) != 1 || d[0].(*api.RolloutConflict).RolloutId != p.Id {
		t.Fatal(d)
	}
	s.tick()
	result := waitState(t, s, p.Id, "FAILED")
	if result.BlocksDeployment || result.Targets[0].Operation.Error != "startup failed" {
		t.Fatal(result)
	}
	if _, err = a.Create(userContext(s), q); err != nil {
		t.Fatal("failure still blocks", err)
	}
	if _, err = a.Act(userContext(s), &api.RolloutAction{Id: p.Id, Action: "retry", ExpectedRevision: result.Revision}); status.Code(err) != codes.Aborted {
		t.Fatal("retry bypassed newer reservation", err)
	}
}
func TestOwnerLossPersistsAcrossJupiterRestart(t *testing.T) {
	peer := &managedPeer{}
	s, p, c, clock := managedFixture(t, peer)
	root := s.Store.Dir
	_, err := (&managementAPI{s: s}).Detach(userContext(s), c)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	replacement := testServer(t, root, peer)
	clock.Add(21000)
	replacement.Now = func() time.Time { return time.UnixMilli(clock.Load()) }
	replacement.tick()
	waitState(t, replacement, p.Id, "CANCELLED")
}
func TestAutomaticReconciliationDoesNotReplayLostStart(t *testing.T) {
	peer := &managedPeer{lose: true}
	s, p, _, clock := managedFixture(t, peer)
	s.tick()
	waitState(t, s, p.Id, "ATTENTION")
	peer.mu.Lock()
	for _, op := range peer.ops {
		op.State = "SUCCEEDED"
	}
	peer.mu.Unlock()
	clock.Add(3001)
	s.tick()
	waitState(t, s, p.Id, "WAITING")
	peer.mu.Lock()
	defer peer.mu.Unlock()
	if peer.starts != 1 {
		t.Fatal(peer.starts)
	}
}
func TestManagementSecretsNeverAppearInRolloutList(t *testing.T) {
	s, _, c, _ := managedFixture(t, &managedPeer{})
	list, err := (&rolloutAPI{s: s}).List(userContext(s), &api.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprint(list), c.Token) {
		t.Fatal("secret in public response")
	}
}

func TestCancelAcceptsReviewedOlderRevision(t *testing.T) {
	s, p, _, _ := managedFixture(t, &managedPeer{})
	result, err := (&rolloutAPI{s: s}).Act(userContext(s), &api.RolloutAction{Id: p.Id, Action: "cancel", ExpectedRevision: 0})
	if err != nil || !result.CancelRequested {
		t.Fatal("progress prevented cancellation", result, err)
	}
	s.tick()
	waitState(t, s, p.Id, "CANCELLED")
}
