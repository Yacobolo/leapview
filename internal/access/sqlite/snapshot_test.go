package sqlite

import "testing"

func TestDecodeAccessPolicyRejectsTrailingJSON(t *testing.T) {
	if _, err := decodeAccessPolicyJSON(`{} {"dataPolicies":{}}`); err == nil {
		t.Fatal("decodeAccessPolicyJSON accepted multiple JSON values")
	}
}
