package deployment

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/artifact"
	"github.com/fatima-go/fatima-core/opm/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type rolloutAPI struct {
	api.UnimplementedDeploymentsServer
	s *Server
}

func (s *Server) targets(ctx context.Context, group string) ([]*api.Target, error) {
	all, e := s.Targets()
	if e != nil {
		return nil, internal(e)
	}
	var targets []*api.Target
	for _, t := range all {
		if group != "" && t.Group != group {
			continue
		}
		t = proto.Clone(t).(*api.Target)
		caps, e := s.Peer.Capabilities(ctx, t.Endpoint)
		switch {
		case e != nil:
			t.Reason = e.Error()
			t.Legacy = errors.Is(e, transport.ErrLegacy)
		case transport.Supports(caps, "unavailable"):
			t.Reason = "Juno v2 unavailable; inspect deployment storage configuration"
		case caps.PackageId != t.PackageId:
			t.Reason = "package identity differs from registration"
		case caps.Server != "juno" || !transport.Supports(caps, "deployment"):
			t.Reason = "deployment API unavailable"
		default:
			t.Supported = true
			t.Platform = caps.Platform
		}
		targets = append(targets, t)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].PackageId < targets[j].PackageId })
	return targets, nil
}
func (a *rolloutAPI) Targets(ctx context.Context, q *api.TargetQuery) (*api.TargetList, error) {
	if _, e := a.s.authorize(ctx, "MONITOR", nil); e != nil {
		return nil, e
	}
	ts, e := a.s.targets(ctx, q.Group)
	return &api.TargetList{Targets: ts}, e
}
func (a *rolloutAPI) Create(ctx context.Context, q *api.CreateRollout) (*api.Rollout, error) {
	actor, e := a.s.authorize(ctx, "OPERATOR", nil)
	if e != nil {
		return nil, e
	}
	if !artifact.SafeName.MatchString(q.RequestId) || q.Group == "" || q.FirstPackageId == "" {
		return nil, status.Error(codes.InvalidArgument, "request ID, group and first package required")
	}
	requestID := actor.Username + ":" + q.RequestId
	hash := requestHash(q)
	var db rolloutDB
	if e = a.s.Store.Update("rollouts", &db, nil); e != nil {
		return nil, internal(e)
	}
	for _, r := range db.Records {
		if r.RequestID == requestID {
			if r.RequestHash != hash {
				return nil, status.Error(codes.AlreadyExists, "request ID conflict")
			}
			return r.Plan, nil
		}
	}
	art, e := a.s.getArtifact(q.ArtifactId)
	if e != nil {
		return nil, e
	}
	if art.ExpiresAt < a.s.Now().Add(ExecutionBudget).Unix() {
		return nil, status.Error(codes.FailedPrecondition, "artifact has less than 30 minutes remaining; upload again with a new request ID")
	}
	ts, e := a.s.targets(ctx, q.Group)
	if e != nil {
		return nil, e
	}
	wanted := map[string]bool{}
	for _, id := range q.PackageIds {
		if wanted[id] {
			return nil, status.Error(codes.InvalidArgument, "duplicate target")
		}
		wanted[id] = true
	}
	p := &api.Rollout{Id: transport.ID("r_"), Artifact: art, Group: q.Group, State: "RUNNING", CreatedBy: actor.Username, CreatedAt: a.s.Now().Unix(), Revision: 1, Message: "deploying first package"}
	foundFirst := false
	for _, t := range ts {
		if len(q.PackageIds) > 0 && !wanted[t.PackageId] {
			continue
		}
		delete(wanted, t.PackageId)
		if !t.Supported {
			return nil, status.Errorf(codes.FailedPrecondition, "%s: %s; no deployment submitted", t.PackageId, t.Reason)
		}
		if len(art.Platforms) > 0 {
			found := false
			for _, v := range art.Platforms {
				if v == t.Platform {
					found = true
				}
			}
			if !found {
				return nil, status.Errorf(codes.FailedPrecondition, "artifact lacks %s for %s", t.Platform, t.PackageId)
			}
		}
		run := &api.TargetRun{Target: t, Operation: &api.Operation{Id: transport.ID("o_"), ArtifactId: art.Id, Sha256: art.Sha256, Process: art.Process, PackageId: t.PackageId, State: "QUEUED"}, TotalBytes: art.Size}
		if t.PackageId == q.FirstPackageId {
			p.Targets = append([]*api.TargetRun{run}, p.Targets...)
			foundFirst = true
		} else {
			p.Targets = append(p.Targets, run)
		}
	}
	if !foundFirst || len(wanted) > 0 {
		return nil, status.Error(codes.InvalidArgument, "selected package is absent from this group")
	}
	db = rolloutDB{}
	e = a.s.Store.Update("rollouts", &db, func() error {
		if db.Records == nil {
			db.Records = map[string]*rolloutRecord{}
		}
		for _, r := range db.Records {
			if r.RequestID == requestID {
				if r.RequestHash != hash {
					return status.Error(codes.AlreadyExists, "request ID conflict")
				}
				p = r.Plan
				return nil
			}
			if active(r.Plan.State) && r.Plan.Group == p.Group && r.Plan.Artifact.Process == art.Process {
				return status.Errorf(codes.Aborted, "active rollout %s already owns this group/process", r.Plan.Id)
			}
		}
		db.Records[p.Id] = &rolloutRecord{Plan: p, RequestID: requestID, RequestHash: hash}
		return nil
	})
	return p, internal(e)
}
func active(state string) bool {
	return state != "SUCCEEDED" && state != "CANCELLED" && state != "EXPIRED"
}
func (s *Server) getRollout(id string) (*api.Rollout, error) {
	var db rolloutDB
	if e := s.Store.Update("rollouts", &db, nil); e != nil {
		return nil, internal(e)
	}
	r := db.Records[id]
	if r == nil {
		return nil, status.Error(codes.NotFound, "rollout not found")
	}
	return r.Plan, nil
}
func (a *rolloutAPI) Get(ctx context.Context, q *api.RolloutQuery) (*api.Rollout, error) {
	if _, e := a.s.authorize(ctx, "MONITOR", nil); e != nil {
		return nil, e
	}
	return a.s.getRollout(q.Id)
}
func (a *rolloutAPI) List(ctx context.Context, _ *api.Empty) (*api.RolloutList, error) {
	if _, e := a.s.authorize(ctx, "MONITOR", nil); e != nil {
		return nil, e
	}
	var db rolloutDB
	if e := a.s.Store.Update("rollouts", &db, nil); e != nil {
		return nil, internal(e)
	}
	result := &api.RolloutList{}
	for _, r := range db.Records {
		result.Rollouts = append(result.Rollouts, r.Plan)
	}
	sort.Slice(result.Rollouts, func(i, j int) bool { return result.Rollouts[i].CreatedAt > result.Rollouts[j].CreatedAt })
	return result, nil
}
func (a *rolloutAPI) Act(ctx context.Context, q *api.RolloutAction) (*api.Rollout, error) {
	if _, e := a.s.authorize(ctx, "OPERATOR", nil); e != nil {
		return nil, e
	}
	var db rolloutDB
	var p *api.Rollout
	e := a.s.Store.Update("rollouts", &db, func() error {
		r := db.Records[q.Id]
		if r == nil {
			return status.Error(codes.NotFound, "rollout not found")
		}
		p = r.Plan
		if p.Revision != q.ExpectedRevision {
			return status.Error(codes.Aborted, "rollout changed; refresh before confirming")
		}
		switch q.Action {
		case "continue":
			if p.State != "WAITING" {
				return status.Error(codes.FailedPrecondition, "rollout is not awaiting approval")
			}
			if p.Artifact.ExpiresAt < a.s.Now().Add(ExecutionBudget).Unix() {
				return status.Error(codes.FailedPrecondition, "artifact has less than 30 minutes remaining")
			}
			p.RemainingApproved = true
			p.State = "RUNNING"
			p.Message = "remaining packages approved; deploying sequentially"
		case "resume":
			if p.State != "ATTENTION" {
				return status.Error(codes.FailedPrecondition, "only an uncertain operation can be reconciled")
			}
			p.State = "RUNNING"
			p.Message = "reconciling the same operation ID"
		case "retry":
			if p.State != "FAILED" {
				return status.Error(codes.FailedPrecondition, "retry requires a definitive failed or interrupted operation")
			}
			if p.Artifact.ExpiresAt < a.s.Now().Add(ExecutionBudget).Unix() {
				return status.Error(codes.FailedPrecondition, "artifact has less than 30 minutes remaining")
			}
			for _, t := range p.Targets {
				if t.Operation.State == "FAILED" || t.Operation.State == "INTERRUPTED" {
					t.PreviousAttempts = append(t.PreviousAttempts, t.Operation)
					t.Operation = proto.Clone(t.Operation).(*api.Operation)
					t.Operation.Id = transport.ID("o_")
					t.Operation.State = "QUEUED"
					t.Operation.Events = nil
					t.Operation.Error = ""
					t.Operation.StartedAt = 0
					t.Operation.FinishedAt = 0
					t.Operation.RevisionPath = ""
					t.Operation.PreviousRevision = ""
					t.SentBytes = 0
					break
				}
			}
			p.State = "RUNNING"
			p.Message = "retry approved"
		case "cancel":
			if !active(p.State) {
				return status.Error(codes.FailedPrecondition, "rollout already finished")
			}
			// An uncertain/running Juno is not forcibly terminated. Reconciliation must
			// establish its result before releasing the group/process reservation.
			p.CancelRequested = true
			if p.State == "ATTENTION" {
				p.State = "RUNNING"
			}
			if p.State != "RUNNING" {
				p.State = "CANCELLED"
			}
			p.Message = "stop requested; any accepted operation finishes before cancellation"
		default:
			return status.Error(codes.InvalidArgument, "unknown action")
		}
		p.Revision++
		r.Owner = ""
		r.LeaseUntil = 0
		return nil
	})
	return p, internal(e)
}
func (a *rolloutAPI) Watch(q *api.WatchRequest, stream grpc.ServerStreamingServer[api.Rollout]) error {
	if _, e := a.s.authorize(stream.Context(), "MONITOR", nil); e != nil {
		return e
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	last := q.AfterRevision
	for {
		p, e := a.s.getRollout(q.Id)
		if e != nil {
			return e
		}
		if p.Revision != last {
			if e = stream.Send(p); e != nil {
				return e
			}
			last = p.Revision
		}
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-a.s.ctx.Done():
			return status.Error(codes.Unavailable, "Jupiter stopping; reconnect with rollout ID")
		case <-ticker.C:
		}
		if _, e := a.s.authorize(stream.Context(), "MONITOR", nil); e != nil {
			return e
		}
	}
}

// Work is owned by durable plans, not by the lifetime of a client RPC.
func (s *Server) StartWorkers() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.tick()
			}
		}
	}()
}
func (s *Server) tick() {
	if e := s.expireArtifacts(); e != nil {
		return
	}
	var db rolloutDB
	var ids []string
	e := s.Store.Update("rollouts", &db, func() error {
		for id, r := range db.Records {
			if r.Plan.State == "WAITING" && r.Plan.Artifact.ExpiresAt <= s.Now().Unix() {
				r.Plan.State = "EXPIRED"
				r.Plan.Message = "artifact expired while awaiting approval"
				r.Plan.Revision++
			}
			if r.Plan.State == "RUNNING" && (r.Owner == s.instance || r.LeaseUntil <= s.Now().Unix()) {
				r.Owner = s.instance
				r.LeaseUntil = s.Now().Add(10 * time.Second).Unix()
				ids = append(ids, id)
			}
		}
		return nil
	})
	if e != nil {
		return
	}
	for _, id := range ids {
		s.mu.Lock()
		if s.running[id] {
			s.mu.Unlock()
			continue
		}
		s.running[id] = true
		s.wg.Add(1)
		s.mu.Unlock()
		go func(id string) {
			defer s.wg.Done()
			defer func() { s.mu.Lock(); delete(s.running, id); s.mu.Unlock() }()
			s.drive(id)
		}(id)
	}
}
func (s *Server) change(id string, fn func(*api.Rollout)) error {
	var db rolloutDB
	return s.Store.Update("rollouts", &db, func() error {
		r := db.Records[id]
		if r == nil || r.Owner != s.instance || r.Plan.State != "RUNNING" {
			return fmt.Errorf("rollout ownership changed")
		}
		before := requestHash(r.Plan)
		fn(r.Plan)
		if before != requestHash(r.Plan) {
			r.Plan.Revision++
		}
		return nil
	})
}
func (s *Server) fail(id, state string, e error) {
	_ = s.change(id, func(p *api.Rollout) { p.State = state; p.Message = e.Error() })
}
func (s *Server) drive(id string) {
	for {
		if s.ctx.Err() != nil {
			return
		}
		p, e := s.getRollout(id)
		if e != nil || p.State != "RUNNING" {
			return
		}
		index := -1
		for i, t := range p.Targets {
			if t.Operation.State != "SUCCEEDED" {
				index = i
				break
			}
		}
		if index < 0 {
			_ = s.change(id, func(p *api.Rollout) { p.State = "SUCCEEDED"; p.Message = "all selected packages deployed" })
			return
		}
		t := p.Targets[index]
		if p.CancelRequested && t.Operation.State == "QUEUED" {
			_ = s.change(id, func(p *api.Rollout) { p.State = "CANCELLED" })
			return
		}
		if index > 0 && !p.RemainingApproved {
			_ = s.change(id, func(p *api.Rollout) {
				p.State = "WAITING"
				p.Message = "first package completed; verify it, then approve remaining packages"
			})
			return
		}
		// Prior successes must still point at this revision. Legacy writes remain
		// possible and are detected here instead of being restricted by v2.
		for i := 0; i < index; i++ {
			previous := p.Targets[i]
			ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
			op, e := s.Peer.Get(ctx, previous.Target.Endpoint, previous.Operation.Id, s.ticket(p, previous))
			cancel()
			if e != nil || op == nil || op.State != "SUCCEEDED" {
				s.fail(id, "ATTENTION", fmt.Errorf("verify previous package %s before continuing: %v", previous.Target.PackageId, e))
				return
			}
		}
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		op, e := s.Peer.Get(ctx, t.Target.Endpoint, t.Operation.Id, s.ticket(p, t))
		cancel()
		if status.Code(e) == codes.NotFound {
			if p.CancelRequested {
				_ = s.change(id, func(p *api.Rollout) { p.State = "CANCELLED" })
				return
			}
			if p.Artifact.ExpiresAt <= s.Now().Unix() {
				s.fail(id, "EXPIRED", fmt.Errorf("artifact expired before dispatch"))
				return
			}
			if p.Artifact.ExpiresAt < s.Now().Add(ExecutionBudget).Unix() {
				s.fail(id, "ATTENTION", fmt.Errorf("artifact has less than 30 minutes remaining; no new operation dispatched"))
				return
			}
			// Persist intent before sending: a lost response must be reconciled using
			// the same operation ID, never replaced with a new deployment request.
			if e = s.change(id, func(p *api.Rollout) { p.Targets[index].Operation.State = "STAGING" }); e != nil {
				return
			}
			spec := &api.OperationSpec{Id: t.Operation.Id, ArtifactId: p.Artifact.Id, Sha256: p.Artifact.Sha256, Size: p.Artifact.Size, Process: p.Artifact.Process, PackageId: t.Target.PackageId, ExpiresAt: p.Artifact.ExpiresAt}
			deadline := s.Now().Add(ExecutionBudget)
			if expiry := time.Unix(p.Artifact.ExpiresAt, 0); expiry.Before(deadline) {
				deadline = expiry
			}
			ctx, cancel = context.WithDeadline(s.ctx, deadline)
			lastProgress := time.Time{}
			op, e = s.Peer.Stage(ctx, t.Target.Endpoint, spec, s.artifactPath(p.Artifact.Id), s.ticket(p, t), func(n int64) {
				if n != p.Artifact.Size && time.Since(lastProgress) < 200*time.Millisecond {
					return
				}
				lastProgress = time.Now()
				_ = s.change(id, func(p *api.Rollout) { p.Targets[index].SentBytes = n })
			})
			cancel()
		}
		if e != nil {
			if s.ctx.Err() != nil {
				return
			}
			s.fail(id, "ATTENTION", fmt.Errorf("%s: result uncertain; resume to reconcile operation %s: %w", t.Target.PackageId, t.Operation.Id, e))
			return
		}
		if op.State == "STAGED" {
			// Fully staged bytes may finish after the artifact retention deadline.
			if p.CancelRequested {
				_ = s.change(id, func(p *api.Rollout) { p.State = "CANCELLED" })
				return
			}
			ctx, cancel = context.WithTimeout(s.ctx, 5*time.Second)
			op, e = s.Peer.Start(ctx, t.Target.Endpoint, t.Operation.Id, s.ticket(p, t))
			cancel()
			if e != nil {
				s.fail(id, "ATTENTION", fmt.Errorf("start response uncertain; resume operation %s: %w", t.Operation.Id, e))
				return
			}
		}
		if op.Id != t.Operation.Id || op.Sha256 != p.Artifact.Sha256 || op.PackageId != t.Target.PackageId {
			s.fail(id, "ATTENTION", fmt.Errorf("Juno operation identity/digest mismatch"))
			return
		}
		if observer, ok := s.Peer.(interface {
			Observe(context.Context, string, string, string, func(*api.Operation) error) error
		}); ok && op.State == "RUNNING" {
			ctx, stop := context.WithTimeout(s.ctx, ExecutionBudget+time.Minute)
			e = observer.Observe(ctx, t.Target.Endpoint, t.Operation.Id, s.ticket(p, t), func(snapshot *api.Operation) error {
				if snapshot.Id != t.Operation.Id || snapshot.Sha256 != p.Artifact.Sha256 || snapshot.PackageId != t.Target.PackageId {
					return fmt.Errorf("Juno operation identity/digest mismatch")
				}
				op = snapshot
				return s.change(id, func(p *api.Rollout) {
					p.Targets[index].Operation = snapshot
					p.Targets[index].SentBytes = p.Artifact.Size
				})
			})
			stop()
			if e != nil {
				if s.ctx.Err() != nil {
					return
				}
				s.fail(id, "ATTENTION", fmt.Errorf("progress stream interrupted; resume operation %s: %w", t.Operation.Id, e))
				return
			}
		}
		if e = s.change(id, func(p *api.Rollout) {
			p.Targets[index].Operation = op
			if op.State != "STAGING" {
				p.Targets[index].SentBytes = p.Artifact.Size
			}
		}); e != nil {
			return
		}
		switch op.State {
		case "SUCCEEDED":
			continue
		case "FAILED", "INTERRUPTED":
			s.fail(id, "FAILED", fmt.Errorf("%s: %s", t.Target.PackageId, op.Error))
			return
		case "RUNNING":
		default:
			s.fail(id, "ATTENTION", fmt.Errorf("%s: unexpected operation state %s", t.Target.PackageId, op.State))
			return
		}
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}
