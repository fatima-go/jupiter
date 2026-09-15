package deployment

import (
	"testing"
	"time"

	"github.com/fatima-go/fatima-core/opm/api"
	"google.golang.org/protobuf/proto"
)

func intervalFixture(t *testing.T) (*Server, *api.Rollout, *api.ManagementCredential, func(int64), *managedPeer) {
	t.Helper()
	peer := &managedPeer{}
	s, p, c, clock := managedFixture(t, peer)
	var db rolloutDB
	if err := s.Store.Update("rollouts", &db, func() error {
		r := db.Records[p.Id]
		r.Owner = s.instance
		r.Plan.RemainingApproved = true
		r.Plan.Targets = append(r.Plan.Targets, &api.TargetRun{Target: &api.Target{PackageId: "c:default", Endpoint: "c:default", CancelSupported: true}, Operation: &api.Operation{Id: "o_third", PackageId: "c:default", Sha256: p.Artifact.Sha256, State: "QUEUED"}})
		r.Plan.Targets[0].Operation.State = "SUCCEEDED"
		r.Plan.Targets[1].Operation.State = "RUNNING"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	p, _ = s.getRollout(p.Id)
	peer.ops = map[string]*api.Operation{}
	for _, target := range p.Targets[:2] {
		peer.ops[target.Operation.Id] = proto.Clone(target.Operation).(*api.Operation)
		peer.ops[target.Operation.Id].State = "SUCCEEDED"
	}
	if err := s.recordOperation(p.Id, peer.ops[p.Targets[1].Operation.Id]); err != nil {
		t.Fatal(err)
	}
	p, _ = s.getRollout(p.Id)
	if p.NextTargetStartAt != clock.Load()+5000 {
		t.Fatal(p)
	}
	return s, p, c, func(ms int64) { clock.Add(ms) }, peer
}

func TestTargetIntervalStopsDispatchAndHonorsOwnerLoss(t *testing.T) {
	for _, action := range []string{"elapsed", "cancel", "detach", "timeout"} {
		t.Run(action, func(t *testing.T) {
			s, p, c, advance, peer := intervalFixture(t)
			s.tick()
			advance(4999)
			time.Sleep(600 * time.Millisecond)
			peer.mu.Lock()
			starts := peer.starts
			peer.mu.Unlock()
			if starts != 0 {
				t.Fatal("next target started before deadline")
			}
			current, _ := s.getRollout(p.Id)
			if current.NextTargetStartAt != p.NextTargetStartAt {
				t.Fatal("deadline changed")
			}
			switch action {
			case "elapsed":
				advance(1)
				deadline := time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) {
					peer.mu.Lock()
					starts = peer.starts
					if starts > 0 {
						peer.ops["o_third"].State = "SUCCEEDED"
					}
					peer.mu.Unlock()
					if starts > 0 {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				result := waitState(t, s, p.Id, "SUCCEEDED")
				if starts != 1 || result.NextTargetStartAt != 0 {
					t.Fatal(result, starts)
				}
			case "cancel":
				if _, err := (&rolloutAPI{s: s}).Act(userContext(s), &api.RolloutAction{Id: p.Id, Action: "cancel"}); err != nil {
					t.Fatal(err)
				}
			case "detach":
				if _, err := (&managementAPI{s: s}).Detach(userContext(s), c); err != nil {
					t.Fatal(err)
				}
			case "timeout":
				advance(15001)
			}
			if action != "elapsed" {
				result := waitState(t, s, p.Id, "CANCELLED")
				peer.mu.Lock()
				defer peer.mu.Unlock()
				if peer.starts != 0 || result.NextTargetStartAt != 0 {
					t.Fatal(result)
				}
			}
		})
	}
}

func TestTargetIntervalPersistsAndDoesNotRestartOnRepeatedSuccess(t *testing.T) {
	s, p, _, advance, peer := intervalFixture(t)
	advance(1000)
	if err := s.recordOperation(p.Id, peer.ops[p.Targets[1].Operation.Id]); err != nil {
		t.Fatal(err)
	}
	s.Close()
	replacement := testServer(t, s.Store.Dir, peer)
	restored, err := replacement.getRollout(p.Id)
	if err != nil || restored.NextTargetStartAt != p.NextTargetStartAt {
		t.Fatal(restored, err)
	}
	replacement.Now = s.Now
	advance(3999)
	replacement.tick()
	time.Sleep(600 * time.Millisecond)
	peer.mu.Lock()
	starts := peer.starts
	peer.mu.Unlock()
	if starts != 0 {
		t.Fatal("restart bypassed persisted deadline")
	}
	advance(1)
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		peer.mu.Lock()
		starts = peer.starts
		peer.mu.Unlock()
		if starts == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("restart reset or lost deadline")
}

func TestTargetIntervalFirstLastAndDisabled(t *testing.T) {
	for _, which := range []string{"first", "last", "disabled"} {
		t.Run(which, func(t *testing.T) {
			s, p, _, _, _ := intervalFixture(t)
			index := 0
			if which == "last" {
				index = 2
			}
			if which == "disabled" {
				index = 1
				s.TargetInterval = 0
			}
			if err := s.change(p.Id, func(p *api.Rollout) { p.NextTargetStartAt = 0; p.Targets[index].Operation.State = "RUNNING" }); err != nil {
				t.Fatal(err)
			}
			op := proto.Clone(p.Targets[index].Operation).(*api.Operation)
			op.State = "SUCCEEDED"
			if err := s.recordOperation(p.Id, op); err != nil {
				t.Fatal(err)
			}
			result, _ := s.getRollout(p.Id)
			if result.NextTargetStartAt != 0 {
				t.Fatal(result)
			}
		})
	}
}
