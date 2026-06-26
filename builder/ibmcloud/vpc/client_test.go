package vpc

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// newTestVpcService builds a vpcv1.VpcV1 pointed at an httptest server with a
// no-op authenticator so the client helpers can be exercised end to end.
func newTestVpcService(t *testing.T, url string) *vpcv1.VpcV1 {
	t.Helper()
	svc, err := vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		Authenticator: stubAuthenticator{},
		URL:           url,
	})
	if err != nil {
		t.Fatalf("failed to build test vpc service: %s", err)
	}
	return svc
}

// TestIsResourceReadyImageStatus pins the status→ready/error mapping. Transient
// HTTP errors are no longer classified here (the SDK's request retries absorb
// them, see TestVPCServiceRetriesTransientErrors); this only covers the
// application-level status handling on a successful response.
func TestIsResourceReadyImageStatus(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantReady bool
		wantErr   bool
	}{
		{name: "available image is ready", body: `{"id":"img-1","status":"available"}`, wantReady: true},
		{name: "pending image is not ready", body: `{"id":"img-1","status":"pending"}`, wantReady: false},
		{name: "failed image is an error", body: `{"id":"img-1","status":"failed"}`, wantReady: false, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			state := new(multistep.BasicStateBag)
			state.Put("vpcService", newTestVpcService(t, srv.URL))
			client := IBMCloudClient{}

			ready, err := client.isResourceReady("img-1", "images", state)
			if ready != tt.wantReady {
				t.Errorf("ready = %v, want %v", ready, tt.wantReady)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestVPCServiceRetriesTransientErrors proves the retry configuration we apply
// in StepCreateVPCServiceInstance (vpcRetryMaxAttempts) makes the IBM SDK ride
// out a transient 5xx and ultimately succeed. The interval is overridden to 1ms
// purely to keep the test fast — it caps the exponential backoff.
func TestVPCServiceRetriesTransientErrors(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if atomic.AddInt32(&calls, 1) <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"img-1","status":"available"}`))
	}))
	defer srv.Close()

	svc := newTestVpcService(t, srv.URL)
	svc.EnableRetries(vpcRetryMaxAttempts, time.Millisecond)

	if _, _, err := svc.GetImage(svc.NewGetImageOptions("img-1")); err != nil {
		t.Fatalf("GetImage failed despite retries: %s", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 requests (2 transient + 1 success), got %d", got)
	}
}

func TestVPCServiceGivesUpAfterRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	svc := newTestVpcService(t, srv.URL)
	svc.EnableRetries(vpcRetryMaxAttempts, time.Millisecond)

	if _, _, err := svc.GetImage(svc.NewGetImageOptions("img-1")); err == nil {
		t.Fatal("expected an error after retries are exhausted")
	}
	// One initial attempt plus vpcRetryMaxAttempts retries.
	if want := int32(vpcRetryMaxAttempts + 1); atomic.LoadInt32(&calls) != want {
		t.Errorf("expected %d requests, got %d", want, atomic.LoadInt32(&calls))
	}
}

// TestWaitForResourceReadyAbortsOnAPIError exercises the production poll loop
// end to end: with no SDK retries on this test client, an API error surfaces
// from the check immediately and pollUntil aborts the wait (in production the
// SDK would have retried transient errors before this point). This locks the
// post-rework semantics — pollUntil no longer rides out errors itself.
func TestWaitForResourceReadyAbortsOnAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("vpcService", newTestVpcService(t, srv.URL))
	client := IBMCloudClient{}

	if err := client.waitForResourceReady("i-1", "instances", 30*time.Second, state); err == nil {
		t.Fatal("expected pollUntil to abort the wait when the API errors")
	}
}

func TestWaitForResourceReadyReturnsWhenReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"i-1","status":"running"}`))
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("vpcService", newTestVpcService(t, srv.URL))
	client := IBMCloudClient{}

	if err := client.waitForResourceReady("i-1", "instances", 30*time.Second, state); err != nil {
		t.Fatalf("waitForResourceReady returned error when the resource was ready: %s", err)
	}
}

func TestVPCServiceDoesNotRetryFatalErrors(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	svc := newTestVpcService(t, srv.URL)
	svc.EnableRetries(vpcRetryMaxAttempts, time.Millisecond)

	if _, _, err := svc.GetImage(svc.NewGetImageOptions("img-1")); err == nil {
		t.Fatal("expected a fatal 404 error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected exactly one request for a fatal 404, got %d", got)
	}
}
