package engine

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/fatima-go/fatima-core/crypt"
	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/jupiter/deployment"
	"github.com/fatima-go/jupiter/domain"
	"github.com/fatima-go/jupiter/service"
)

func (server *JupiterHttpServer) newDeploymentV2(interactor *service.DomainInteractor) (*deployment.Server, error) {
	guide := server.fatimaRuntime.GetEnv().GetFolderGuide()
	root := filepath.Join(guide.GetDataFolder(), "deployment-v2")
	if v, ok := server.fatimaRuntime.GetConfig().GetValue("deployment.v2.storage"); ok && v != "" {
		root = v
	}
	login := func(user, password string) (string, error) {
		plain := crypt.ResolveSecret(password)
		role, e := interactor.ValidateUser(domain.User{Id: user, Password: plain})
		return role.String(), e
	}
	// Read the persisted registration snapshot. Never mutate the legacy
	// repository or share its cached objects with v2 worker goroutines.
	targets := func() ([]*api.Target, error) {
		b, e := os.ReadFile(filepath.Join(guide.GetDataFolder(), "juno.json"))
		if os.IsNotExist(e) {
			return nil, nil
		}
		if e != nil {
			return nil, e
		}
		var summary domain.JunoSummary
		if e = json.Unmarshal(b, &summary); e != nil {
			return nil, e
		}
		var result []*api.Target
		for _, g := range summary.Groups {
			for _, p := range g.Packages {
				result = append(result, &api.Target{PackageId: p.Host + ":" + p.Name, Group: g.Name, Endpoint: p.Endpoint})
			}
		}
		return result, nil
	}
	s, err := deployment.New(root, login, targets)
	if err != nil {
		return nil, err
	}
	s.Inventory = func() ([]*api.PackageEntry, error) {
		b, e := os.ReadFile(filepath.Join(guide.GetDataFolder(), "juno.json"))
		if os.IsNotExist(e) {
			return nil, nil
		}
		if e != nil {
			return nil, e
		}
		var summary domain.JunoSummary
		if e = json.Unmarshal(b, &summary); e != nil {
			return nil, e
		}
		var result []*api.PackageEntry
		for _, g := range summary.Groups {
			for _, p := range g.Packages {
				var registered int64
				if v, ok := p.RegistDate.(float64); ok {
					registered = int64(v)
				}
				result = append(result, &api.PackageEntry{Target: &api.Target{PackageId: p.Host + ":" + p.Name, Group: g.Name, Endpoint: p.Endpoint, Platform: p.Platform.Os + "_" + p.Platform.Architecture}, RegisteredAt: registered})
			}
		}
		return result, nil
	}
	return s, nil
}
