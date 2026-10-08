package vpc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// TestStepCreateVPCServiceInstanceEnablesRetries guards the wiring: transient-
// error tolerance for every VPC call depends on the EnableRetries call in Run.
// The behavioral retry tests (client_test.go) enable retries themselves, so they
// would still pass if that line were removed — this test fails if it is.
func TestStepCreateVPCServiceInstanceEnablesRetries(t *testing.T) {
	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("client", &IBMCloudClient{IBMApiKey: "dummy-key"})
	state.Put("config", Config{IBMApiKey: "dummy-key"})

	step := &StepCreateVPCServiceInstance{}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("Run returned %v, want ActionContinue", action)
	}

	svc := state.Get("vpcService").(*vpcv1.VpcV1)
	// EnableRetries swaps the service's HTTP client for a retryablehttp-backed
	// one; assert that transport is in place so dropping EnableRetries is caught.
	if got := fmt.Sprintf("%T", svc.Service.Client.Transport); !strings.Contains(got, "retryablehttp") {
		t.Errorf("VPC service transport = %s, want a retryablehttp transport (EnableRetries not wired)", got)
	}
}

// twoStepServer returns a test HTTP server that serves two sequential POST
// requests. The first call (apikey grant) returns step1Body with step1Status;
// the second call (iam-authz grant) returns step2Body with step2Status.
func twoStepServer(t *testing.T, step1Status int, step1Body, step2Body string) *httptest.Server {
	t.Helper()
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			w.WriteHeader(step1Status)
			fmt.Fprint(w, step1Body)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, step2Body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestNewAuthenticatorServiceAPIKeyHappyPath verifies the full two-step flow:
// apikey grant → intermediate token → iam-authz grant → scoped token →
// BearerTokenAuthenticator.
func TestNewAuthenticatorServiceAPIKeyHappyPath(t *testing.T) {
	srv := twoStepServer(t,
		http.StatusOK, `{"access_token":"intermediate-token"}`,
		`{"access_token":"scoped-token"}`,
	)

	cfg := Config{
		IAMServiceAPIKey: "test-api-key",
		DesiredIAMID:     "crn:v1:test:::",
		IAMEndpoint:      srv.URL,
	}

	auth, err := newAuthenticator(cfg)
	if err != nil {
		t.Fatalf("newAuthenticator returned error: %v", err)
	}
	bearer, ok := auth.(*core.BearerTokenAuthenticator)
	if !ok {
		t.Fatalf("expected *core.BearerTokenAuthenticator, got %T", auth)
	}
	if bearer.BearerToken != "scoped-token" {
		t.Errorf("BearerToken = %q, want %q", bearer.BearerToken, "scoped-token")
	}
}

// TestNewAuthenticatorServiceAPIKeyStep1Failure verifies that a non-200 from
// the apikey grant causes newAuthenticator to return an error.
func TestNewAuthenticatorServiceAPIKeyStep1Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"errorCode":"BXNIM0415E","errorMessage":"Provided API key could not be found"}`)
	}))
	t.Cleanup(srv.Close)

	cfg := Config{
		IAMServiceAPIKey: "bad-key",
		DesiredIAMID:     "crn:v1:test:::",
		IAMEndpoint:      srv.URL,
	}

	_, err := newAuthenticator(cfg)
	if err == nil {
		t.Fatal("expected error from step 1 failure, got nil")
	}
	if !strings.Contains(err.Error(), "iam_service_api_key") {
		t.Errorf("error %q does not mention iam_service_api_key", err.Error())
	}
}

// TestNewAuthenticatorServiceAPIKeyStep2Failure verifies that a non-200 from
// the iam-authz grant causes newAuthenticator to return an error.
func TestNewAuthenticatorServiceAPIKeyStep2Failure(t *testing.T) {
	srv := twoStepServer(t,
		http.StatusOK, `{"access_token":"intermediate-token"}`,
		// step2Body is unused because step2Status is non-200 — but twoStepServer
		// always returns 200 for call 2; use a single-call server instead.
		``,
	)
	srv.Close() // replace with a custom two-call server below

	call := 0
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"access_token":"intermediate-token"}`)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"errorCode":"BXNIM0501E","errorMessage":"Not authorized"}`)
	}))
	defer srv2.Close()

	cfg := Config{
		IAMServiceAPIKey: "test-api-key",
		DesiredIAMID:     "crn:v1:test:::",
		IAMEndpoint:      srv2.URL,
	}

	_, err := newAuthenticator(cfg)
	if err == nil {
		t.Fatal("expected error from step 2 failure, got nil")
	}
	if !strings.Contains(err.Error(), "desired_iam_id") {
		t.Errorf("error %q does not mention desired_iam_id", err.Error())
	}
}
