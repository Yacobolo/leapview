package client_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flidai/leapview/rtest/internal/client"
	"github.com/flidai/leapview/rtest/internal/protocol"
)

func TestStreamReconnectsFromLastEventID(t *testing.T) {
	var requests atomic.Int32
	finished := protocol.Job{ID: "job-1", Status: protocol.StatusSucceeded, CreatedAt: time.Now()}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/jobs/job-1/logs" {
			attempt := requests.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			if attempt == 1 {
				fmt.Fprintf(w, "id: 6\nevent: log\ndata: %s\n\n", base64.StdEncoding.EncodeToString([]byte("hello\n")))
				return
			}
			if got := r.Header.Get("Last-Event-ID"); got != "6" {
				t.Errorf("Last-Event-ID = %q, want 6", got)
			}
			data, _ := json.Marshal(finished)
			fmt.Fprintf(w, "id: 6\nevent: done\ndata: %s\n\n", data)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	api, err := client.New(server.URL, "token")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var output bytes.Buffer
	job, err := api.Stream(ctx, "job-1", &output)
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 || output.String() != "hello\n" || job.Status != protocol.StatusSucceeded {
		t.Fatalf("requests=%d output=%q job=%#v", requests.Load(), output.String(), job)
	}
}
