package deployment

import (
	"context"
	"fmt"
	"time"

	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/lifecycle"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type cancelPeer interface {
	Cancel(context.Context, string, *api.OperationSpec, string) (*api.Operation, error)
}

func operationSpec(p *api.Rollout, t *api.TargetRun) *api.OperationSpec {
	return &api.OperationSpec{Id: t.Operation.Id, ArtifactId: p.Artifact.Id, Sha256: p.Artifact.Sha256, Size: p.Artifact.Size, Process: p.Artifact.Process, PackageId: t.Target.PackageId, ExpiresAt: p.Artifact.ExpiresAt}
}

// Before every dispatch, persist intent and recheck ownership and the CLI lease.
// A stale worker cannot create new intent after owner expiry or cancellation.
func (s *Server) dispatch(id, op, state string) (bool, error) {
	var db rolloutDB
	allowed := false
	err := s.Store.Update("rollouts", &db, func() error {
		expireManagement(&db, s.Now().UnixMilli())
		r := db.Records[id]
		if r == nil || r.Owner != s.instance || r.Plan.State != "RUNNING" {
			return nil
		}
		if op == "" {
			allowed = true
			return nil
		}
		if r.Plan.CancelRequested {
			return nil
		}
		for _, t := range r.Plan.Targets {
			if t.Operation.Id == op {
				t.Operation.State = state
				r.Plan.Revision++
				allowed = true
				return nil
			}
		}
		return fmt.Errorf("operation no longer belongs to rollout")
	})
	return allowed, err
}
func (s *Server) recordOperation(id string, op *api.Operation) error {
	return s.change(id, func(p *api.Rollout) {
		for _, t := range p.Targets {
			if t.Operation.Id == op.Id {
				t.Operation = op
				break
			}
		}
		p.LastCheckedAt = s.Now().UnixMilli()
		p.NextCheckAt = 0
	})
}
func (s *Server) stopAfterFailure(id string, t *api.TargetRun, err error) {
	_ = s.change(id, func(p *api.Rollout) {
		for _, r := range p.Targets {
			if r.Operation.Id == t.Operation.Id {
				r.Operation.State = "FAILED"
				r.Operation.Error = err.Error()
				r.Operation.FinishedAt = s.Now().Unix()
			}
		}
		p.CancelRequested = true
		p.EndReason = "DEPLOYMENT_FAILED"
		p.Message = err.Error()
	})
}
func knownRejection(err error) bool {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.PermissionDenied, codes.Unauthenticated, codes.FailedPrecondition:
		return true
	}
	return false
}
func (s *Server) drive(id string) {
	for s.ctx.Err() == nil {
		// Expiry is checked even if tick was delayed by storage maintenance.
		if ok, err := s.dispatch(id, "", "RUNNING"); err != nil || !ok {
			return
		}
		p, err := s.getRollout(id)
		if err != nil || p.State != "RUNNING" {
			return
		}
		if p.CancelRequested {
			done, err := s.cancelRemaining(p)
			if err != nil {
				s.fail(id, "ATTENTION", err)
				return
			}
			if done {
				return
			}
			if !s.pause() {
				return
			}
			continue
		}
		index := -1
		for i, t := range p.Targets {
			if t.Operation.State != "SUCCEEDED" {
				index = i
				break
			}
		}
		if index < 0 {
			_ = s.change(id, func(p *api.Rollout) { p.State = "SUCCEEDED"; p.Message = "전체 배포 완료" })
			return
		}
		t := p.Targets[index]
		if lifecycle.OperationTerminal(t.Operation.State) {
			s.stopAfterFailure(id, t, fmt.Errorf("%s: %s", t.Target.PackageId, t.Operation.Error))
			continue
		}
		if index > 0 && !p.RemainingApproved {
			_ = s.change(id, func(p *api.Rollout) {
				p.State = "WAITING"
				p.Message = "첫 서버 완료 · 나머지 서버 배포 승인 필요"
			})
			return
		}
		// Previously completed revisions must still match before another deployment.
		for i := 0; i < index; i++ {
			prev := p.Targets[i]
			ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
			op, e := s.Peer.Get(ctx, prev.Target.Endpoint, prev.Operation.Id, s.ticket(p, prev))
			cancel()
			if e != nil || op == nil || op.State != "SUCCEEDED" {
				s.fail(id, "ATTENTION", fmt.Errorf("이전 서버 결과 확인 필요: %s: %v", prev.Target.PackageId, e))
				return
			}
		}
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		op, err := s.Peer.Get(ctx, t.Target.Endpoint, t.Operation.Id, s.ticket(p, t))
		cancel()
		if status.Code(err) == codes.NotFound {
			if p.Artifact.ExpiresAt < s.Now().Add(ExecutionBudget).Unix() {
				_ = s.change(id, func(p *api.Rollout) {
					p.CancelRequested = true
					p.EndReason = "ARTIFACT_EXPIRED"
					p.Message = "배포 파일 만료 · 미실행 작업 정리"
				})
				continue
			}
			ok, e := s.dispatch(id, t.Operation.Id, "STAGING")
			if e != nil {
				return
			}
			if !ok {
				continue
			}
			ctx, cancel = context.WithTimeout(s.ctx, ExecutionBudget)
			// A cancelled/expired owner interrupts upload, not an executing process.
			stop := s.watchTransferOwner(ctx, id, cancel)
			op, err = s.Peer.Stage(ctx, t.Target.Endpoint, operationSpec(p, t), s.artifactPath(p.Artifact.Id), s.ticket(p, t), func(n int64) {
				_ = s.change(id, func(p *api.Rollout) {
					for _, r := range p.Targets {
						if r.Operation.Id == t.Operation.Id {
							r.SentBytes = n
						}
					}
				})
			})
			close(stop)
			cancel()
			if err != nil && knownRejection(err) {
				s.stopAfterFailure(id, t, err)
				continue
			}
		} else if err != nil && t.Operation.State == "QUEUED" && knownRejection(err) {
			s.stopAfterFailure(id, t, err)
			continue
		}
		if err != nil {
			s.fail(id, "ATTENTION", fmt.Errorf("서버 응답 확인 중 · 같은 작업을 자동 조회합니다: %w", err))
			return
		}
		if op == nil || op.Id != t.Operation.Id || op.PackageId != t.Target.PackageId || op.Sha256 != p.Artifact.Sha256 {
			s.fail(id, "ATTENTION", fmt.Errorf("operation identity/digest mismatch"))
			return
		}
		if op.State == "STAGED" {
			ok, e := s.dispatch(id, t.Operation.Id, "STARTING")
			if e != nil {
				return
			}
			if !ok {
				continue
			}
			ctx, cancel = context.WithTimeout(s.ctx, 5*time.Second)
			op, err = s.Peer.Start(ctx, t.Target.Endpoint, t.Operation.Id, s.ticket(p, t))
			cancel()
			if err != nil {
				// Start may already have arrived. Seal/query it, never infer termination.
				s.fail(id, "ATTENTION", fmt.Errorf("실행 응답 확인 중: %w", err))
				return
			}
			if op == nil || op.Id != t.Operation.Id || op.PackageId != t.Target.PackageId || op.Sha256 != p.Artifact.Sha256 {
				s.fail(id, "ATTENTION", fmt.Errorf("start operation identity/digest mismatch"))
				return
			}
		}
		if err = s.recordOperation(id, op); err != nil {
			return
		}
		if op.State == "SUCCEEDED" {
			continue
		}
		if lifecycle.OperationTerminal(op.State) {
			s.stopAfterFailure(id, t, fmt.Errorf("%s", op.Error))
			continue
		}
		if op.State != "RUNNING" {
			s.fail(id, "ATTENTION", fmt.Errorf("unexpected Juno state %s", op.State))
			return
		}
		if !s.pause() {
			return
		}
	}
}
func (s *Server) pause() bool {
	select {
	case <-s.ctx.Done():
		return false
	case <-time.After(500 * time.Millisecond):
		return true
	}
}
func (s *Server) watchTransferOwner(ctx context.Context, id string, cancel context.CancelFunc) chan struct{} {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				p, err := s.getRollout(id)
				if err != nil || p.CancelRequested || p.State != "RUNNING" {
					cancel()
					return
				}
			}
		}
	}()
	return stop
}
func (s *Server) cancelRemaining(p *api.Rollout) (bool, error) {
	pending := false
	for _, t := range p.Targets {
		if lifecycle.OperationTerminal(t.Operation.State) {
			continue
		}
		var op *api.Operation
		var err error
		if t.Operation.State == "QUEUED" {
			// No dispatch intent was ever committed for this ID.
			op = t.Operation
			op.State = "CANCELLED"
			op.FinishedAt = s.Now().Unix()
		} else {
			ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
			if peer, ok := s.Peer.(cancelPeer); ok && t.Target.CancelSupported {
				op, err = peer.Cancel(ctx, t.Target.Endpoint, operationSpec(p, t), s.ticket(p, t))
			} else {
				op, err = s.Peer.Get(ctx, t.Target.Endpoint, t.Operation.Id, s.ticket(p, t))
				if err == nil && op != nil && !lifecycle.OperationTerminal(op.State) && op.State != "RUNNING" {
					err = fmt.Errorf("Juno 업데이트 필요: 미실행 작업 취소 API 미지원")
				}
			}
			cancel()
			if err != nil {
				return false, fmt.Errorf("취소 처리 중 · %s 결과 재확인: %w", t.Target.PackageId, err)
			}
		}
		if op == nil || op.Id != t.Operation.Id || op.PackageId != t.Target.PackageId || op.Sha256 != p.Artifact.Sha256 {
			return false, fmt.Errorf("cancel operation identity/digest mismatch")
		}
		if err = s.recordOperation(p.Id, op); err != nil {
			return false, err
		}
		if !lifecycle.OperationTerminal(op.State) {
			pending = true
		}
	}
	if pending {
		return false, nil
	}
	err := s.change(p.Id, func(p *api.Rollout) {
		failed, success, cancelled := false, 0, 0
		for _, t := range p.Targets {
			switch t.Operation.State {
			case "FAILED", "INTERRUPTED", "DRIFTED":
				failed = true
			case "SUCCEEDED":
				success++
			case "CANCELLED":
				cancelled++
			}
		}
		switch {
		case failed:
			p.State = "FAILED"
		case success == len(p.Targets):
			p.State = "SUCCEEDED"
		case p.EndReason == "ARTIFACT_EXPIRED":
			p.State = "EXPIRED"
		default:
			p.State = "CANCELLED"
		}
		p.Message = fmt.Sprintf("배포 종료 · 완료 %d · 미실행 취소 %d · 실패 여부 %t · 새 배포 가능", success, cancelled, failed)
		p.NextCheckAt = 0
	})
	return err == nil, err
}
