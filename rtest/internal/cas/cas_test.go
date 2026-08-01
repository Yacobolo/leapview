package cas

import (
	"context"
	"strings"
	"testing"
)

func TestMaterializeRejectsMalformedRootDigestBeforeDialing(t *testing.T) {
	err := Materialize(context.Background(), "unused:1", "rtest", "not-a-digest", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "parse REAPI root digest") {
		t.Fatalf("error = %v", err)
	}
}
