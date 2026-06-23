package vpc

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// newTestVpcService builds a vpcv1.VpcV1 pointed at an httptest server with a no-op
// authenticator so the resource-polling helpers can be exercised end to end.
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

func TestIsTransientPollError(t *testing.T) {
	tests := []struct {
		name string
		resp *core.DetailedResponse
		err  error
		want bool
	}{
		{name: "no error is not transient", resp: &core.DetailedResponse{StatusCode: 200}, err: nil, want: false},
		{name: "500 is transient", resp: &core.DetailedResponse{StatusCode: 500}, err: errors.New("boom"), want: true},
		{name: "502 is transient", resp: &core.DetailedResponse{StatusCode: 502}, err: errors.New("bad gateway"), want: true},
		{name: "503 is transient", resp: &core.DetailedResponse{StatusCode: 503}, err: errors.New("unavailable"), want: true},
		{name: "504 is transient", resp: &core.DetailedResponse{StatusCode: 504}, err: errors.New("timeout"), want: true},
		{name: "429 is transient", resp: &core.DetailedResponse{StatusCode: 429}, err: errors.New("rate limited"), want: true},
		{name: "404 is fatal", resp: &core.DetailedResponse{StatusCode: 404}, err: errors.New("not found"), want: false},
		{name: "401 is fatal", resp: &core.DetailedResponse{StatusCode: 401}, err: errors.New("unauthorized"), want: false},
		{name: "403 is fatal", resp: &core.DetailedResponse{StatusCode: 403}, err: errors.New("forbidden"), want: false},
		{name: "400 is fatal", resp: &core.DetailedResponse{StatusCode: 400}, err: errors.New("bad request"), want: false},
		{name: "network error with no response is transient", resp: nil, err: errors.New("connection reset by peer"), want: true},
		{name: "response with no status code is transient", resp: &core.DetailedResponse{StatusCode: 0}, err: errors.New("eof"), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransientPollError(tt.resp, tt.err); got != tt.want {
				t.Fatalf("isTransientPollError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsResourceReadyImageClassification(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          string
		wantReady     bool
		wantErr       bool
		wantTransient bool
	}{
		{name: "available image is ready", status: http.StatusOK, body: `{"id":"img-1","status":"available"}`, wantReady: true},
		{name: "pending image is not ready", status: http.StatusOK, body: `{"id":"img-1","status":"pending"}`, wantReady: false},
		{name: "failed image is an error", status: http.StatusOK, body: `{"id":"img-1","status":"failed"}`, wantReady: false, wantErr: true},
		{name: "502 is a transient error", status: http.StatusBadGateway, body: `{}`, wantErr: true, wantTransient: true},
		{name: "503 is a transient error", status: http.StatusServiceUnavailable, body: `{}`, wantErr: true, wantTransient: true},
		{name: "404 is a fatal error", status: http.StatusNotFound, body: `{}`, wantErr: true, wantTransient: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
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
			var tpe *transientPollError
			if got := errors.As(err, &tpe); got != tt.wantTransient {
				t.Errorf("errors.As transientPollError = %v, want %v (err: %v)", got, tt.wantTransient, err)
			}
		})
	}
}

func TestWaitForResourceReadyToleratesTransientFailures(t *testing.T) {
	var calls int32
	// Return three transient 502s, then report the image as available.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if atomic.AddInt32(&calls, 1) <= 3 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"img-1","status":"available"}`))
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("vpcService", newTestVpcService(t, srv.URL))
	client := IBMCloudClient{pollInterval: time.Millisecond}

	if err := client.waitForResourceReady("img-1", "images", 30*time.Second, state); err != nil {
		t.Fatalf("waitForResourceReady returned error after transient failures: %s", err)
	}
}

func TestWaitForResourceReadyGivesUpAfterTooManyTransientFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("vpcService", newTestVpcService(t, srv.URL))
	client := IBMCloudClient{pollInterval: time.Millisecond}

	err := client.waitForResourceReady("img-1", "images", 30*time.Second, state)
	if err == nil {
		t.Fatal("expected an error after exceeding the consecutive transient failure cap")
	}
	if !strings.Contains(err.Error(), "transient") {
		t.Errorf("error should mention transient failures, got: %s", err)
	}
}

func TestWaitForResourceReadyResetsTransientStreak(t *testing.T) {
	// 5x502 (the full tolerated streak), then a successful-but-not-ready poll
	// that resets the counter, then 5x502 again, then available. Ten transient
	// errors total but never 6 in a row, so the build must NOT abort. This pins
	// both the cap (5 consecutive tolerated) and the reset-on-success behavior.
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		n := atomic.AddInt32(&calls, 1)
		switch {
		case n <= 5: // first transient burst (5 tolerated)
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{}`))
		case n == 6: // successful poll, not ready yet -> resets the streak
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"img-1","status":"pending"}`))
		case n <= 11: // second transient burst (5 more)
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"img-1","status":"available"}`))
		}
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("vpcService", newTestVpcService(t, srv.URL))
	client := IBMCloudClient{pollInterval: time.Millisecond}

	if err := client.waitForResourceReady("img-1", "images", 30*time.Second, state); err != nil {
		t.Fatalf("waitForResourceReady aborted despite the transient streak resetting: %s", err)
	}
}

func TestIsResourceReadyTransientClassificationByType(t *testing.T) {
	for _, resourceType := range []string{"floating_ips", "subnets"} {
		tests := []struct {
			name          string
			status        int
			wantTransient bool
		}{
			{name: "502 is transient", status: http.StatusBadGateway, wantTransient: true},
			{name: "404 is fatal", status: http.StatusNotFound, wantTransient: false},
		}
		for _, tt := range tests {
			t.Run(resourceType+" "+tt.name, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.status)
					_, _ = w.Write([]byte(`{}`))
				}))
				defer srv.Close()

				state := new(multistep.BasicStateBag)
				state.Put("vpcService", newTestVpcService(t, srv.URL))
				client := IBMCloudClient{}

				_, err := client.isResourceReady("r-1", resourceType, state)
				if err == nil {
					t.Fatalf("expected an error for status %d", tt.status)
				}
				var tpe *transientPollError
				if got := errors.As(err, &tpe); got != tt.wantTransient {
					t.Errorf("errors.As transientPollError = %v, want %v (err: %v)", got, tt.wantTransient, err)
				}
			})
		}
	}
}

func TestIsResourceDownClassification(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          string
		wantDown      bool
		wantErr       bool
		wantTransient bool
	}{
		{name: "stopped instance is down", status: http.StatusOK, body: `{"id":"i-1","status":"stopped"}`, wantDown: true},
		{name: "running instance is not down", status: http.StatusOK, body: `{"id":"i-1","status":"running"}`, wantDown: false},
		{name: "502 is a transient error", status: http.StatusBadGateway, body: `{}`, wantErr: true, wantTransient: true},
		{name: "404 is a fatal error", status: http.StatusNotFound, body: `{}`, wantErr: true, wantTransient: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			state := new(multistep.BasicStateBag)
			state.Put("ui", packer.TestUi(t))
			state.Put("vpcService", newTestVpcService(t, srv.URL))
			client := IBMCloudClient{}

			down, err := client.isResourceDown("i-1", "instances", state)

			if down != tt.wantDown {
				t.Errorf("down = %v, want %v", down, tt.wantDown)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			var tpe *transientPollError
			if got := errors.As(err, &tpe); got != tt.wantTransient {
				t.Errorf("errors.As transientPollError = %v, want %v (err: %v)", got, tt.wantTransient, err)
			}
		})
	}
}

func TestWaitForResourceDownToleratesTransientFailures(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if atomic.AddInt32(&calls, 1) <= 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"i-1","status":"stopped"}`))
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("vpcService", newTestVpcService(t, srv.URL))
	client := IBMCloudClient{pollInterval: time.Millisecond}

	if err := client.waitForResourceDown("i-1", "instances", 30*time.Second, state); err != nil {
		t.Fatalf("waitForResourceDown returned error after transient failures: %s", err)
	}
}

func TestWaitForResourceDownGivesUpAfterTooManyTransientFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("vpcService", newTestVpcService(t, srv.URL))
	client := IBMCloudClient{pollInterval: time.Millisecond}

	err := client.waitForResourceDown("i-1", "instances", 30*time.Second, state)
	if err == nil {
		t.Fatal("expected an error after exceeding the consecutive transient failure cap")
	}
	if !strings.Contains(err.Error(), "transient") {
		t.Errorf("error should mention transient failures, got: %s", err)
	}
}

func TestWaitForResourceReadyFatalErrorFailsImmediately(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("vpcService", newTestVpcService(t, srv.URL))
	client := IBMCloudClient{pollInterval: time.Millisecond}

	if err := client.waitForResourceReady("img-1", "images", 30*time.Second, state); err == nil {
		t.Fatal("expected a fatal error for a 404 response")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected exactly one poll for a fatal error, got %d", got)
	}
}
