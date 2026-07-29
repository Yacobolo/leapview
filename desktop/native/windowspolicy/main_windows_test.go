package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSecureAdministratorPathRejectsUserWritableACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-policy.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	admin, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		t.Fatal(err)
	}
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		t.Fatal(err)
	}
	users, err := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	if err != nil {
		t.Fatal(err)
	}
	setPolicySecurity(t, path, system, []windows.EXPLICIT_ACCESS{
		grant(admin, windows.GENERIC_ALL),
		grant(system, windows.GENERIC_ALL),
		grant(users, windows.GENERIC_READ),
	})
	if err := secureAdministratorPath(path); err != nil {
		t.Fatalf("expected secure policy: %v", err)
	}
	setPolicySecurity(t, path, system, []windows.EXPLICIT_ACCESS{
		grant(admin, windows.GENERIC_ALL),
		grant(system, windows.GENERIC_ALL),
		grant(users, windows.GENERIC_WRITE),
	})
	if err := secureAdministratorPath(path); err == nil {
		t.Fatal("expected user-writable policy to be rejected")
	}
}

func setPolicySecurity(
	t *testing.T,
	path string,
	owner *windows.SID,
	entries []windows.EXPLICIT_ACCESS,
) {
	t.Helper()
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|
			windows.DACL_SECURITY_INFORMATION|
			windows.PROTECTED_DACL_SECURITY_INFORMATION,
		owner,
		nil,
		acl,
		nil,
	); err != nil {
		t.Fatal(err)
	}
}

func grant(
	sid *windows.SID,
	permissions windows.ACCESS_MASK,
) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: permissions,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_GROUP,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}
