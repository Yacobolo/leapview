package instanceidentity

import "testing"

func TestValidAcceptsDurableAndDesktopInstanceIDs(t *testing.T) {
	for _, value := range []string{
		"lvinst_0123456789abcdefghijklmnopqrstuv",
		"lvinst_0123456789ABCDEFGHIJKLMNOPQRSTUV",
		"instance_0123456789abcdef0123456789abcdef",
	} {
		if !Valid(value) {
			t.Fatalf("Valid(%q) = false, want true", value)
		}
	}
}

func TestValidRejectsMalformedInstanceIDs(t *testing.T) {
	for _, value := range []string{
		"",
		"lvinst_short",
		"lvinst_0123456789abcdefghijklmnopqrstu!",
		"instance_0123456789ABCDEF0123456789ABCDEF",
		"instance_0123456789abcdef0123456789abcdef0",
	} {
		if Valid(value) {
			t.Fatalf("Valid(%q) = true, want false", value)
		}
	}
}
