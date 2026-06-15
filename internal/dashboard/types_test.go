package dashboard

import "testing"

func TestTableRequestWithDefaultsClampsRuntimePolicy(t *testing.T) {
	request := TableRequest{
		Table:      "orders",
		Block:      "z",
		Start:      -20,
		Count:      TableMaxRequestCount + 500,
		RequestSeq: -8,
		Sort:       TableSort{Key: "revenue", Direction: "sideways"},
	}.WithDefaults()

	if request.Block != "all" {
		t.Fatalf("block = %q, want all", request.Block)
	}
	if request.Start != 0 {
		t.Fatalf("start = %d, want 0", request.Start)
	}
	if request.Count != TableMaxRequestCount {
		t.Fatalf("count = %d, want %d", request.Count, TableMaxRequestCount)
	}
	if request.RequestSeq != 0 {
		t.Fatalf("request seq = %d, want 0", request.RequestSeq)
	}
	if request.Sort.Direction != "desc" {
		t.Fatalf("sort direction = %q, want desc", request.Sort.Direction)
	}
}

func TestTableRequestResetRequestsInitialBlocks(t *testing.T) {
	request := TableRequest{
		Table:        "orders",
		Block:        "b",
		Start:        600,
		Count:        500,
		RequestSeq:   42,
		ResetVersion: 4,
		Sort:         TableSort{Key: "revenue", Direction: "asc"},
	}.Reset()

	if request.Block != "all" || request.Start != 0 || request.Count != TableChunkSize {
		t.Fatalf("reset request = %#v, want all at top with chunk size", request)
	}
	if request.ResetVersion != 5 {
		t.Fatalf("reset version = %d, want 5", request.ResetVersion)
	}
	if request.RequestSeq != 42 {
		t.Fatalf("request seq = %d, want 42", request.RequestSeq)
	}
	if request.Sort.Key != "revenue" || request.Sort.Direction != "asc" {
		t.Fatalf("reset sort = %#v, want preserved revenue asc", request.Sort)
	}
}
