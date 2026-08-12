package vpc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type subnetInfo struct{ vpc, zone string }

// subnetHandler serves GET /v1/subnets/{id} from a fixed catalog, so a test can
// drive stepGetSubnetInfo over several subnets with controlled VPC/zone values.
func subnetHandler(catalog map[string]subnetInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		info, ok := catalog[path.Base(r.URL.Path)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"subnet not found"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"id":%q,"vpc":{"id":%q},"zone":{"name":%q},"status":"available"}`,
			path.Base(r.URL.Path), info.vpc, info.zone)
	}
}

func newSubnetInfoState(t *testing.T, url string, subnetIDs ...string) *multistep.BasicStateBag {
	t.Helper()
	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("vpcService", newTestVpcService(t, url))
	state.Put("config", Config{SubnetIDs: subnetIDs})
	return state
}

// TestStepGetSubnetInfoSameVPC covers the multi-subnet golden path: every subnet
// resolves, they share a VPC, and bake_subnets is populated with each subnet's
// zone (order is shuffled, so it's compared as a set).
func TestStepGetSubnetInfoSameVPC(t *testing.T) {
	srv := httptest.NewServer(subnetHandler(map[string]subnetInfo{
		"0717-a": {vpc: "vpc-1", zone: "us-east-1"},
		"0727-b": {vpc: "vpc-1", zone: "us-east-2"},
	}))
	defer srv.Close()

	state := newSubnetInfoState(t, srv.URL, "0717-a", "0727-b")

	step := &stepGetSubnetInfo{}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("Run action = %v, want ActionContinue (err=%v)", action, state.Get("error"))
	}
	if got := state.Get("vpc_id"); got != "vpc-1" {
		t.Errorf("vpc_id = %v, want vpc-1", got)
	}
	bake, ok := state.Get("bake_subnets").([]subnetZone)
	if !ok || len(bake) != 2 {
		t.Fatalf("bake_subnets = %v, want 2 entries", state.Get("bake_subnets"))
	}
	gotZones := map[string]string{}
	for _, sz := range bake {
		gotZones[sz.ID] = sz.Zone
	}
	for id, wantZone := range map[string]string{"0717-a": "us-east-1", "0727-b": "us-east-2"} {
		if gotZones[id] != wantZone {
			t.Errorf("subnet %s zone = %q, want %q", id, gotZones[id], wantZone)
		}
	}
}

// TestStepGetSubnetInfoRejectsCrossVPC pins the same-VPC guard: a second subnet
// in a different VPC halts the build, since the VPC identity and security group
// are fixed and a cross-VPC subnet can't serve as a fallback.
func TestStepGetSubnetInfoRejectsCrossVPC(t *testing.T) {
	srv := httptest.NewServer(subnetHandler(map[string]subnetInfo{
		"0717-a": {vpc: "vpc-1", zone: "us-east-1"},
		"0727-b": {vpc: "vpc-2", zone: "us-east-2"},
	}))
	defer srv.Close()

	state := newSubnetInfoState(t, srv.URL, "0717-a", "0727-b")

	step := &stepGetSubnetInfo{}
	if action := step.Run(context.Background(), state); action != multistep.ActionHalt {
		t.Fatalf("Run action = %v, want ActionHalt", action)
	}
	err, _ := state.Get("error").(error)
	if err == nil || !strings.Contains(err.Error(), "same VPC") {
		t.Fatalf("error = %v, want it to mention same VPC", err)
	}
}
