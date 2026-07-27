package instancelock

import "testing"

func TestAcquireRejectsSecondProcessForSameHome(t *testing.T) {
	home := t.TempDir()
	first, err := Acquire(home)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if _, err := Acquire(home); err == nil {
		t.Fatal("second lock acquisition succeeded")
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(home)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
}

func TestAcquireNamedUsesAnIndependentSafeLockFile(t *testing.T) {
	home := t.TempDir()
	instance, err := Acquire(home)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Release()
	named, err := AcquireNamed(home, ".first-login.lock")
	if err != nil {
		t.Fatal(err)
	}
	defer named.Release()
	if _, err := AcquireNamed(home, ".first-login.lock"); err == nil {
		t.Fatal("second named lock acquisition succeeded")
	}
	for _, invalid := range []string{"", "../escape", "nested/lock"} {
		if _, err := AcquireNamed(home, invalid); err == nil {
			t.Fatalf("unsafe lock name %q accepted", invalid)
		}
	}
}
