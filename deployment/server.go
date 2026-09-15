// Package deployment is the additive v2 deployment service. Legacy handlers do
// not call this package and retain their original contracts and behavior.
package deployment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/store"
	"github.com/fatima-go/fatima-core/opm/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const ArtifactTTL = 24 * time.Hour
const ExecutionBudget = 30 * time.Minute

type Peer interface {
	Capabilities(context.Context, string) (*api.Capabilities, error)
	Stage(context.Context, string, *api.OperationSpec, string, string, func(int64)) (*api.Operation, error)
	Start(context.Context, string, string, string) (*api.Operation, error)
	Get(context.Context, string, string, string) (*api.Operation, error)
}

type Server struct {
	Inventory      func() ([]*api.PackageEntry, error)
	inventoryMu    sync.Mutex
	inventoryCache *api.PackageCatalog
	inventoryKey   string
	Store          *store.Store
	LoginUser      func(string, string) (string, error)
	Targets        func() ([]*api.Target, error)
	Peer           Peer
	Now            func() time.Time
	TargetInterval time.Duration // Configured before starting workers; zero disables the gap.
	instance       string
	key            []byte
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.Mutex
	running        map[string]bool
	wg             sync.WaitGroup
}

type artifactRecord struct {
	Artifact  *api.Artifact
	RequestID string
}
type artifactDB struct{ Records map[string]*artifactRecord }
type rolloutRecord struct {
	Plan        *api.Rollout
	RequestID   string
	RequestHash string
	Owner       string
	LeaseUntil  int64
}
type rolloutDB struct {
	Records  map[string]*rolloutRecord
	Sessions map[string]*managementRecord
}
type claims struct {
	Username    string
	Role        string
	ExpiresAt   int64
	OperationID string
	SHA256      string
	PackageID   string
}

func New(root string, login func(string, string) (string, error), targets func() ([]*api.Target, error)) (*Server, error) {
	st, e := store.Open(root)
	if e != nil {
		return nil, e
	}
	s := &Server{Store: st, LoginUser: login, Targets: targets, Peer: grpcPeer{}, Now: time.Now, TargetInterval: 5 * time.Second, instance: transport.ID("jupiter_"), running: map[string]bool{}}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	var key struct{ Value string }
	e = st.Update("signing-key", &key, func() error {
		if key.Value == "" {
			key.Value = transport.ID("") + transport.ID("")
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	s.key = []byte(key.Value)
	if e = os.MkdirAll(filepath.Join(root, "artifacts"), 0700); e != nil {
		return nil, e
	}
	return s, nil
}

func (s *Server) Register(g *grpc.Server) {
	api.RegisterDeploymentManagementServer(g, &managementAPI{s: s})
	api.RegisterProcessRegistryServer(g, &registryProxy{s: s})
	api.RegisterPackageInventoryServer(g, &inventoryAPI{s: s})
	api.RegisterRoutingServer(g, &routingAPI{s: s})
	api.RegisterIdentityServer(g, &identityAPI{s: s})
	api.RegisterArtifactsServer(g, &artifactAPI{s: s})
	api.RegisterDeploymentsServer(g, &rolloutAPI{s: s})
}
func (s *Server) Capabilities() *api.Capabilities {
	return &api.Capabilities{Server: "jupiter", ApiVersion: 2, Features: []string{"artifacts", "rollouts", "progress", "resume", "routing", "ropack", "roproc", "deployment_management"}, InstanceId: s.instance, ArtifactTtlSeconds: int64(ArtifactTTL / time.Second)}
}
func (s *Server) Close() { s.cancel(); s.wg.Wait() }

func (s *Server) sign(c claims) string {
	b, _ := json.Marshal(c)
	body := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Server) authorize(ctx context.Context, role string, scope *api.ValidateRequest) (claims, error) {
	token := transport.Token(ctx)
	var c claims
	parts := splitToken(token)
	if len(parts) != 2 {
		return c, status.Error(codes.Unauthenticated, "login required")
	}
	signature, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil {
		return c, status.Error(codes.Unauthenticated, "invalid token")
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return c, status.Error(codes.Unauthenticated, "invalid token")
	}
	b, e := base64.RawURLEncoding.DecodeString(parts[0])
	if e != nil || json.Unmarshal(b, &c) != nil || c.ExpiresAt <= s.Now().Unix() {
		return c, status.Error(codes.Unauthenticated, "token expired")
	}
	if c.Role != "OPERATOR" && (role != "MONITOR" || c.Role != "MONITOR") {
		return c, status.Error(codes.PermissionDenied, "insufficient role")
	}
	if scope != nil && scope.OperationId != "" {
		if c.OperationID != scope.OperationId || (scope.Sha256 != "" && c.SHA256 != scope.Sha256) || c.PackageID != scope.PackageId {
			return c, status.Error(codes.PermissionDenied, "deployment ticket scope mismatch")
		}
	} else if c.OperationID != "" {
		return c, status.Error(codes.PermissionDenied, "deployment ticket cannot invoke user APIs")
	}
	return c, nil
}
func splitToken(s string) []string {
	for i := range s {
		if s[i] == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return nil
}
func (s *Server) ticket(p *api.Rollout, t *api.TargetRun) string {
	return s.sign(claims{Username: p.CreatedBy, Role: "OPERATOR", ExpiresAt: s.Now().Add(ExecutionBudget + time.Minute).Unix(), OperationID: t.Operation.Id, SHA256: p.Artifact.Sha256, PackageID: t.Target.PackageId})
}
func (s *Server) artifactPath(id string) string {
	return filepath.Join(s.Store.Dir, "artifacts", id+".far")
}
func requestHash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type identityAPI struct {
	api.UnimplementedIdentityServer
	s *Server
}

func (a *identityAPI) Login(ctx context.Context, r *api.LoginRequest) (*api.Session, error) {
	if r.Username == "" || len(r.Password) > 4096 {
		return nil, status.Error(codes.InvalidArgument, "invalid credentials")
	}
	role, e := a.s.LoginUser(r.Username, r.Password)
	if e != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication failed")
	}
	if role != "MONITOR" && role != "OPERATOR" {
		return nil, status.Error(codes.PermissionDenied, "unknown role")
	}
	c := claims{Username: r.Username, Role: role, ExpiresAt: a.s.Now().Add(time.Hour).Unix()}
	return &api.Session{Token: a.s.sign(c), Username: c.Username, Role: c.Role, ExpiresAt: c.ExpiresAt}, nil
}
func (a *identityAPI) Validate(ctx context.Context, r *api.ValidateRequest) (*api.Session, error) {
	c, e := a.s.authorize(ctx, r.Role, r)
	if e != nil {
		return nil, e
	}
	return &api.Session{Username: c.Username, Role: c.Role, ExpiresAt: c.ExpiresAt}, nil
}

func internal(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	return status.Error(codes.Internal, fmt.Sprintf("deployment storage: %v", err))
}
