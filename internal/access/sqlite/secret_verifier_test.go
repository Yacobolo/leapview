package sqlite

import "testing"

func TestSecretFingerprintReadsConfiguredKeyAtOperationTime(t *testing.T) {
	t.Setenv("LEAPVIEW_TOKEN_HASH_KEY", "first-runtime-key")
	first := secretFingerprint("same-secret")
	t.Setenv("LEAPVIEW_TOKEN_HASH_KEY", "second-runtime-key")
	second := secretFingerprint("same-secret")
	if first == second {
		t.Fatal("secret fingerprint retained the process-start key after runtime configuration changed")
	}
}
