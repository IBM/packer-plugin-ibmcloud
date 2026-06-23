package vpc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

func TestWaitForExportJobToleratesTransientFailures(t *testing.T) {
	var calls int32
	// Three transient 502s, then the export job reports success.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if atomic.AddInt32(&calls, 1) <= 3 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"succeeded"}`))
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))

	err := waitForExportJobToSucceed("img-1", "job-1", newTestVpcService(t, srv.URL), 30*time.Second, time.Millisecond, state)
	if err != nil {
		t.Fatalf("waitForExportJobToSucceed returned error after transient failures: %s", err)
	}
}

func TestWaitForExportJobGivesUpAfterTooManyTransientFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))

	err := waitForExportJobToSucceed("img-1", "job-1", newTestVpcService(t, srv.URL), 30*time.Second, time.Millisecond, state)
	if err == nil {
		t.Fatal("expected an error after exceeding the consecutive transient failure cap")
	}
	if !strings.Contains(err.Error(), "transient") {
		t.Errorf("error should mention transient failures, got: %s", err)
	}
}

func TestWaitForExportJobFatalErrorFailsImmediately(t *testing.T) {
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
