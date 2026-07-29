package cliapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileStorePersistsOnlyNonSecretTargetMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cli.json")
	store := NewProfileStore(path)
	profile := TargetProfile{
		Origin:            "https://analytics.example.com/",
		InstanceID:        "lvinst_01",
		Environment:       "production",
		CredentialAccount: "target/lvinst_01/project",
		ProjectID:         "project",
	}
	if err := store.Put("prod", profile); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.Origin != "https://analytics.example.com" || got.InstanceID != profile.InstanceID || got.ProjectID != profile.ProjectID {
		t.Fatalf("profile = %+v", got)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"accessToken", "refreshToken", "password", "secret"} {
		if strings.Contains(strings.ToLower(string(content)), strings.ToLower(forbidden)) {
			t.Fatalf("profile contains secret-bearing field %q: %s", forbidden, content)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("profile mode = %o, want 600", info.Mode().Perm())
	}
}

func TestProfileStoreRejectsLegacyPlaintextCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cli.json")
	content := `{"version":1,"targets":{"prod":{"origin":"https://example.test","instanceId":"lvinst_01","token":"plaintext"}}}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewProfileStore(path).Get("prod"); err == nil || !strings.Contains(err.Error(), "secret-bearing") {
		t.Fatalf("Get error = %v, want secret-bearing field rejection", err)
	}
}

func TestProfileStoreRejectsMutableOrUnsafeTargetIdentity(t *testing.T) {
	store := NewProfileStore(filepath.Join(t.TempDir(), "cli.json"))
	tests := []TargetProfile{
		{Origin: "http://analytics.example.com", InstanceID: "lvinst_01", CredentialAccount: "account", ProjectID: "project"},
		{Origin: "https://user:password@example.com", InstanceID: "lvinst_01", CredentialAccount: "account", ProjectID: "project"},
		{Origin: "https://example.com/path", InstanceID: "lvinst_01", CredentialAccount: "account", ProjectID: "project"},
		{Origin: "https://example.com", InstanceID: "", CredentialAccount: "account", ProjectID: "project"},
	}
	for index, profile := range tests {
		if err := store.Put("prod", profile); err == nil {
			t.Errorf("case %d accepted unsafe profile %+v", index, profile)
		}
	}
	if err := store.Put("local", TargetProfile{
		Origin:            "http://127.0.0.1:8080",
		InstanceID:        "lvinst_local",
		Environment:       "development",
		CredentialAccount: "account",
		ProjectID:         "project",
	}); err != nil {
		t.Fatalf("loopback development target rejected: %v", err)
	}
}

func TestProfileStoreRefusesInstanceIdentityReplacement(t *testing.T) {
	store := NewProfileStore(filepath.Join(t.TempDir(), "cli.json"))
	first := TargetProfile{
		Origin:            "https://example.com",
		InstanceID:        "lvinst_first",
		CredentialAccount: "account-first",
		ProjectID:         "project",
	}
	if err := store.Put("prod", first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.InstanceID = "lvinst_second"
	second.CredentialAccount = "account-second"
	if err := store.Put("prod", second); err == nil || !strings.Contains(err.Error(), "instance identity") {
		t.Fatalf("Put error = %v, want immutable instance identity rejection", err)
	}
}

func TestProfileStoreFindsStableNameByCanonicalOrigin(t *testing.T) {
	store := NewProfileStore(filepath.Join(t.TempDir(), "cli.json"))
	if err := store.Put("production", TargetProfile{
		Origin: "https://example.com", InstanceID: "lvinst_prod", CredentialAccount: "account", ProjectID: "project",
	}); err != nil {
		t.Fatal(err)
	}
	name, profile, err := store.FindByOrigin("https://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	if name != "production" || profile.InstanceID != "lvinst_prod" {
		t.Fatalf("name=%q profile=%+v", name, profile)
	}
}
