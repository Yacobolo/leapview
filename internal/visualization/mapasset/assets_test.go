package mapasset

import "testing"

func TestResolveReturnsImmutableSameOriginManifest(t *testing.T) {
	asset, err := Resolve("streets")
	if err != nil {
		t.Fatal(err)
	}
	if asset.ID != "libredash-streets" || asset.StyleURL[0] != '/' || asset.ArchiveURL[0] != '/' || len(asset.ArchiveDigest) != 71 {
		t.Fatalf("asset = %#v", asset)
	}
	if _, err := Resolve("remote"); err == nil {
		t.Fatal("unknown map style asset accepted")
	}
}
