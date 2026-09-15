package deployment

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"

	"github.com/fatima-go/fatima-opm/api"
	"github.com/fatima-go/fatima-opm/lifecycle"
	"github.com/fatima-go/fatima-opm/transport"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type managementRecord struct {
	Session             *api.ManagementSession
	Username, TokenHash string
}
type managementAPI struct {
	api.UnimplementedDeploymentManagementServer
	s *Server
}

func credentialHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func (a *managementAPI) Open(ctx context.Context, _ *api.Empty) (*api.ManagementSession, error) {
	actor, err := a.s.authorize(ctx, "OPERATOR", nil)
	if err != nil {
		return nil, err
	}
	secret := transport.ID("") + transport.ID("")
	now := a.s.Now().UnixMilli()
	session := &api.ManagementSession{Id: transport.ID("m_"), LastSeenAt: now, ExpiresAt: now + lifecycle.OwnerTimeout.Milliseconds(), State: "ACTIVE"}
	var db rolloutDB
	err = a.s.Store.Update("rollouts", &db, func() error {
		if db.Sessions == nil {
			db.Sessions = map[string]*managementRecord{}
		}
		db.Sessions[session.Id] = &managementRecord{Session: session, Username: actor.Username, TokenHash: credentialHash(secret)}
		return nil
	})
	if err != nil {
		return nil, internal(err)
	}
	// Secrets are never stored in public snapshots or returned by Heartbeat/Get.
	out := proto.Clone(session).(*api.ManagementSession)
	out.Token = secret
	return out, nil
}
func managementOwner(db *rolloutDB, q *api.ManagementCredential, user string) (*managementRecord, error) {
	if q == nil || q.Id == "" || q.Token == "" {
		return nil, status.Error(codes.PermissionDenied, "management credential required")
	}
	r := db.Sessions[q.Id]
	if r == nil || r.Username != user || subtle.ConstantTimeCompare([]byte(r.TokenHash), []byte(credentialHash(q.Token))) != 1 {
		return nil, status.Error(codes.PermissionDenied, "invalid management credential")
	}
	return r, nil
}
func closeManagement(db *rolloutDB, r *managementRecord, reason string) {
	if r.Session.State != "ACTIVE" {
		return
	}
	r.Session.State = reason
	if rec := db.Records[r.Session.RolloutId]; rec != nil && !lifecycle.Terminal(rec.Plan.State) {
		p := rec.Plan
		p.NextTargetStartAt = 0
		p.CancelRequested = true
		p.EndReason = reason
		p.State = "RUNNING"
		p.NextCheckAt = 0
		p.Message = "CLI 종료로 남은 배포 취소 중 · 실행 결과 확인"
		p.Revision++
	}
}
func expireManagement(db *rolloutDB, now int64) {
	for _, r := range db.Sessions {
		if r.Session.State == "ACTIVE" && now >= r.Session.ExpiresAt {
			closeManagement(db, r, "OWNER_TIMEOUT")
		}
	}
}
func (a *managementAPI) Heartbeat(ctx context.Context, q *api.ManagementCredential) (*api.ManagementSession, error) {
	return a.update(ctx, q, false)
}
func (a *managementAPI) Detach(ctx context.Context, q *api.ManagementCredential) (*api.ManagementSession, error) {
	return a.update(ctx, q, true)
}
func (a *managementAPI) update(ctx context.Context, q *api.ManagementCredential, detach bool) (*api.ManagementSession, error) {
	actor, err := a.s.authorize(ctx, "OPERATOR", nil)
	if err != nil {
		return nil, err
	}
	var db rolloutDB
	var result *api.ManagementSession
	err = a.s.Store.Update("rollouts", &db, func() error {
		r, e := managementOwner(&db, q, actor.Username)
		if e != nil {
			return e
		}
		now := a.s.Now().UnixMilli()
		if r.Session.State == "ACTIVE" && now >= r.Session.ExpiresAt {
			closeManagement(&db, r, "OWNER_TIMEOUT")
		}
		if detach {
			closeManagement(&db, r, "OWNER_DETACHED")
		} else if r.Session.State == "ACTIVE" {
			r.Session.LastSeenAt = now
			r.Session.ExpiresAt = now + lifecycle.OwnerTimeout.Milliseconds()
		}
		result = r.Session
		return nil
	})
	return result, internal(err)
}
func conflict(p *api.Rollout) error {
	st := status.New(codes.Aborted, "같은 대상에 기존 배포가 있습니다. 기존 작업을 확인하세요.")
	detailed, err := st.WithDetails(&api.RolloutConflict{RolloutId: p.Id, Group: p.Group, Process: p.Artifact.Process})
	if err != nil {
		return st.Err()
	}
	return detailed.Err()
}
func publicRollout(p *api.Rollout) *api.Rollout {
	p.BlocksDeployment = !lifecycle.Terminal(p.State)
	return p
}

func publicRolloutIfPresent(p *api.Rollout) *api.Rollout {
	if p == nil {
		return nil
	}
	return publicRollout(p)
}
