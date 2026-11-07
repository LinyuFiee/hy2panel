package config

import "testing"

func TestRackNerdPublicConfigClearsCredentials(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.RackNerd.APIKey = "secret-key"
	cfg.RackNerd.APIHash = "secret-hash"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	public := cfg.Public()
	if public.RackNerd.APIKey != "" || public.RackNerd.APIHash != "" {
		t.Fatal("public config exposed RackNerd credentials")
	}
	if !cfg.RackNerd.Configured() {
		t.Fatal("original credentials were modified")
	}
}

func TestIncompleteRackNerdCredentialsRejected(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.RackNerd.APIKey = "only-key"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected incomplete credentials to fail")
	}
}
