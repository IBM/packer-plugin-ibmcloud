package vpc

import (
	"context"
	"fmt"
	"strings"
	"testing"

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
	state.Put("config", Config{})

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
