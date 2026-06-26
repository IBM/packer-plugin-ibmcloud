package vpc

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

func TestWaitForExportJobSucceeds(t *testing.T) {
	var calls int32
	// Report "running" twice, then "succeeded" — exercises the poll-again loop.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if atomic.AddInt32(&calls, 1) <= 2 {
			_, _ = w.Write([]byte(`{"status":"running"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"succeeded"}`))
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))

	if err := waitForExportJobToSucceed("img-1", "job-1", newTestVpcService(t, srv.URL), 30*time.Second, time.Millisecond, state); err != nil {
		t.Fatalf("waitForExportJobToSucceed returned error: %s", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 polls (2 running + 1 succeeded), got %d", got)
	}
}

func TestWaitForExportJobFailsOnFailedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"failed"}`))
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))

	if err := waitForExportJobToSucceed("img-1", "job-1", newTestVpcService(t, srv.URL), 30*time.Second, time.Millisecond, state); err == nil {
		t.Fatal("expected an error for a failed export job")
	}
}

// A non-transient API error (e.g. 404) surfaces immediately; transient 5xx/429
// blips are absorbed beneath this function by the SDK's request retries.
func TestWaitForExportJobFailsOnAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))

	if err := waitForExportJobToSucceed("img-1", "job-1", newTestVpcService(t, srv.URL), 30*time.Second, time.Millisecond, state); err == nil {
		t.Fatal("expected a fatal error for a 404 response")
	}
}
