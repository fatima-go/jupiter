package deployment

import (
	"context"
	"testing"

	"github.com/fatima-go/fatima-opm/api"
	"github.com/fatima-go/fatima-opm/transport"
)

func TestLoginReturnsProfile(t *testing.T) {
	login := func(string, string) (string, error) { return "OPERATOR", nil }
	for _, profile := range []string{"qa", ""} {
		s, e := New(t.TempDir(), login, nil)
		if e != nil {
			t.Fatal(e)
		}
		s.Profile = profile
		got, e := (&identityAPI{s: s}).Login(context.Background(), &api.LoginRequest{Username: "admin", Password: "secret"})
		s.Close()
		if e != nil {
			t.Fatal(e)
		}
		if got.Profile != profile {
			t.Fatalf("profile = %q, want %q", got.Profile, profile)
		}
	}
}

// An empty profile is not encoded by proto3, so clients rely on this feature to
// tell an unset FATIMA_PROFILE apart from a Jupiter that never reports one.
func TestCapabilitiesAdvertiseSessionProfile(t *testing.T) {
	s, e := New(t.TempDir(), nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if !transport.Supports(s.Capabilities(), "session_profile") {
		t.Fatal("session_profile feature is not advertised")
	}
}
