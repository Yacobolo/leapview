package configspec

import "testing"

func TestDynamicEnvironmentPrefixesAreNarrowAndDevelopmentOnly(t *testing.T) {
	prefixes := DynamicEnvironmentPrefixes()
	if len(prefixes) != 1 || prefixes[0] != "LEAPVIEW_DEV_CONNECTION_" {
		t.Fatalf("dynamic environment prefixes = %#v", prefixes)
	}
	if knownDynamicEnvironmentReference("LEAPVIEW_DATABASE_URL") ||
		!knownDynamicEnvironmentReference("LEAPVIEW_DEV_CONNECTION_WAREHOUSE") {
		t.Fatal("dynamic environment prefix scope is not fail-closed")
	}
}
