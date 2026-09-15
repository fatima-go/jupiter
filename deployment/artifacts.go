package deployment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatima-go/fatima-opm/api"
	"github.com/fatima-go/fatima-opm/artifact"
	"github.com/fatima-go/fatima-opm/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type artifactAPI struct {
	api.UnimplementedArtifactsServer
	s *Server
}

func (a *artifactAPI) Upload(stream grpc.ClientStreamingServer[api.UploadChunk, api.Artifact]) error {
	actor, e := a.s.authorize(stream.Context(), "OPERATOR", nil)
	if e != nil {
		return e
	}
	first, e := stream.Recv()
	if e != nil {
		return status.Error(codes.InvalidArgument, "upload header required")
	}
	h := first.Header
	if h == nil || len(first.Data) != 0 || !artifact.SafeName.MatchString(h.RequestId) || h.Size <= 0 || h.Size > artifact.MaxSize || len(h.Sha256) != 64 {
		return status.Error(codes.InvalidArgument, "invalid upload header")
	}
	var existing *api.Artifact
	var db artifactDB
	e = a.s.Store.Update("artifacts", &db, nil)
	if e != nil {
		return internal(e)
	}
	for _, r := range db.Records {
		if r.RequestID == actor.Username+":"+h.RequestId {
			existing = r.Artifact
		}
	}
	if existing != nil {
		if existing.Sha256 != h.Sha256 || existing.Size != h.Size {
			return status.Error(codes.AlreadyExists, "request ID belongs to another artifact")
		}
		return stream.SendAndClose(existing)
	}
	file, e := os.CreateTemp(filepath.Join(a.s.Store.Dir, "artifacts"), ".upload-")
	if e != nil {
		return internal(e)
	}
	defer os.Remove(file.Name())
	defer file.Close()
	hash := sha256.New()
	var n int64
	for {
		chunk, e := stream.Recv()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if chunk.Header != nil || len(chunk.Data) == 0 || n+int64(len(chunk.Data)) > h.Size {
			return status.Error(codes.InvalidArgument, "invalid upload chunk")
		}
		written, e := io.MultiWriter(file, hash).Write(chunk.Data)
		if e != nil {
			return internal(e)
		}
		n += int64(written)
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	if n != h.Size || sum != h.Sha256 {
		return status.Error(codes.DataLoss, "upload size or SHA-256 mismatch")
	}
	if e = file.Sync(); e != nil {
		return internal(e)
	}
	if e = file.Close(); e != nil {
		return internal(e)
	}
	info, e := artifact.Inspect(file.Name())
	if e != nil {
		return status.Error(codes.InvalidArgument, e.Error())
	}
	sort.Strings(info.Platforms)
	now := a.s.Now()
	value := &api.Artifact{Id: transport.ID("a_"), Filename: filepath.Base(h.Filename), Process: info.Process, Sha256: sum, Size: n, Platforms: info.Platforms, BuildJson: info.BuildJSON, UploadedBy: actor.Username, UploadedAt: now.Unix(), ExpiresAt: now.Add(ArtifactTTL).Unix()}
	db = artifactDB{}
	e = a.s.Store.Update("artifacts", &db, func() error {
		if db.Records == nil {
			db.Records = map[string]*artifactRecord{}
		}
		for _, r := range db.Records {
			if r.RequestID == actor.Username+":"+h.RequestId {
				if r.Artifact.Sha256 != sum {
					return status.Error(codes.AlreadyExists, "request ID conflict")
				}
				value = r.Artifact
				return nil
			}
		}
		if e := os.Rename(file.Name(), a.s.artifactPath(value.Id)); e != nil {
			return e
		}
		db.Records[value.Id] = &artifactRecord{Artifact: value, RequestID: actor.Username + ":" + h.RequestId}
		return nil
	})
	if e != nil {
		return internal(e)
	}
	return stream.SendAndClose(value)
}

func (a *artifactAPI) List(ctx context.Context, q *api.ArtifactQuery) (*api.ArtifactList, error) {
	if _, e := a.s.authorize(ctx, "MONITOR", nil); e != nil {
		return nil, e
	}
	var db artifactDB
	if e := a.s.Store.Update("artifacts", &db, nil); e != nil {
		return nil, internal(e)
	}
	list := &api.ArtifactList{}
	for _, r := range db.Records {
		v := r.Artifact
		if q.Process != "" && q.Process != v.Process {
			continue
		}
		v.Expired = v.ExpiresAt <= a.s.Now().Unix()
		list.Artifacts = append(list.Artifacts, v)
	}
	sort.Slice(list.Artifacts, func(i, j int) bool { return list.Artifacts[i].UploadedAt > list.Artifacts[j].UploadedAt })
	return list, nil
}
func (a *artifactAPI) Get(ctx context.Context, q *api.ArtifactQuery) (*api.Artifact, error) {
	if _, e := a.s.authorize(ctx, "MONITOR", nil); e != nil {
		return nil, e
	}
	return a.s.getArtifact(q.Id)
}
func (s *Server) getArtifact(id string) (*api.Artifact, error) {
	var db artifactDB
	if e := s.Store.Update("artifacts", &db, nil); e != nil {
		return nil, internal(e)
	}
	r := db.Records[id]
	if r == nil {
		return nil, status.Error(codes.NotFound, "artifact not found")
	}
	r.Artifact.Expired = r.Artifact.ExpiresAt <= s.Now().Unix()
	return r.Artifact, nil
}
func (s *Server) expireArtifacts() error {
	var db artifactDB
	e := s.Store.Update("artifacts", &db, func() error {
		for _, r := range db.Records {
			if r.Artifact.ExpiresAt <= s.Now().Unix() {
				r.Artifact.Expired = true
				if e := os.Remove(s.artifactPath(r.Artifact.Id)); e != nil && !os.IsNotExist(e) {
					return fmt.Errorf("expire artifact: %w", e)
				}
			}
		}
		return nil
	})
	if e != nil {
		return e
	}
	entries, e := os.ReadDir(filepath.Join(s.Store.Dir, "artifacts"))
	if e != nil {
		return e
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || (!strings.HasPrefix(name, ".upload-") && !strings.HasPrefix(name, "a_")) {
			continue
		}
		if db.Records[strings.TrimSuffix(name, ".far")] != nil {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if info.ModTime().Before(s.Now().Add(-ArtifactTTL)) {
			if e = os.Remove(filepath.Join(s.Store.Dir, "artifacts", name)); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
	}
	return nil
}
