package config

import "testing"

func TestValidateProductionAuthRequiresCSRFKey(t *testing.T) {
	cfg := Config{Production: true, APITokenOnlyAuth: true}
	if err := cfg.ValidateProductionAuth(); err == nil {
		t.Fatal("expected missing CSRF key to fail production auth validation")
	}
}

func TestValidateProductionAuthAllowsDevBypassWithoutCSRFKey(t *testing.T) {
	cfg := Config{Production: true, DevAuthBypass: true}
	if err := cfg.ValidateProductionAuth(); err != nil {
		t.Fatalf("validate production auth: %v", err)
	}
}

func TestCookieSecureDefaultsToProduction(t *testing.T) {
	secure, err := (Config{Production: true}).CookieSecure()
	if err != nil {
		t.Fatalf("cookie secure: %v", err)
	}
	if !secure {
		t.Fatal("production cookie secure default = false, want true")
	}
}
